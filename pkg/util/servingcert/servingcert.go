package servingcert

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// CAKey is the Secret key holding the PEM CA.
	CAKey = "ca"
	// CAKeyKey is the Secret key holding the PEM private key of the CA.
	CAKeyKey = "ca.key"
	// CertKey is the Secret key holding the PEM serving cert.
	CertKey = "tls.crt"
	// KeyKey is the Secret key holding the PEM private key of the serving cert.
	KeyKey = "tls.key"
	// PreviousCAKey is the Secret key holding the CA that was replaced last,
	// trusted until the time in PreviousCAUntilAnnotation.
	PreviousCAKey = "ca.previous"
	// PreviousCAUntilAnnotation is the RFC 3339 time until which PreviousCAKey
	// stays in the CA bundle.
	PreviousCAUntilAnnotation = "servingcert.oam.dev/previous-ca-until"

	// Validity is how long a generated CA is valid.
	Validity = 10 * 365 * 24 * time.Hour
	// ServingValidity is how long a generated serving cert is valid.
	ServingValidity = 365 * 24 * time.Hour
	// RenewBefore is how long before expiry a cert or CA is replaced.
	RenewBefore = 30 * 24 * time.Hour
	// CAOverlap is how long a replaced CA stays trusted, allowing servers time
	// to reload the new serving cert without breaking client TLS connections.
	CAOverlap = 24 * time.Hour
)

// Service is the Service a serving cert is for.
type Service struct {
	Namespace string
	Name      string
}

// RequiredHosts are the names that clients dial. EnsureSecret reuses a cert
// only if it covers all of them.
func (s Service) RequiredHosts() []string {
	return []string{s.Name, s.Name + "." + s.Namespace + ".svc"}
}

// Hosts are the names put in a generated cert, including standard cluster DNS,
// localhost and 127.0.0.1 for local health checks and loopback access.
func (s Service) Hosts() []string {
	return append(s.RequiredHosts(),
		s.Name+"."+s.Namespace+".svc.cluster.local",
		"localhost",
		"127.0.0.1",
	)
}

// Result is what EnsureSecret returns.
type Result struct {
	// Cert and Key are the PEM serving cert and its private key.
	Cert, Key []byte
	// CABundle is what clients should trust: the CA, plus the previous CA while
	// it is still within CAOverlap.
	CABundle []byte
}

// EnsureSecret returns the serving cert for svc from the Secret <namespace>/<name>.
// It creates, renews or replaces the cert as necessary. It is safe to call from
// multiple replicas concurrently.
func EnsureSecret(ctx context.Context, c client.Client, namespace, name string, svc Service) (*Result, error) {
	var result *Result
	isRace := func(err error) bool { return apierrors.IsConflict(err) || apierrors.IsAlreadyExists(err) }
	err := retry.OnError(retry.DefaultRetry, isRace, func() error {
		now := time.Now()
		secret := &corev1.Secret{}
		err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, secret)
		if err != nil && !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get secret %s/%s", namespace, name)
		}
		found := err == nil
		if found {
			// Cert must be valid now and still valid at renewal deadline.
			reason := Check(secret.Data, svc.RequiredHosts(), now)
			if reason == nil {
				reason = Check(secret.Data, svc.RequiredHosts(), now.Add(RenewBefore))
			}
			if reason == nil {
				result = resultFor(secret, now)
				return nil
			}
			klog.InfoS("Renewing serving cert", "secret", klog.KObj(secret), "reason", reason.Error())
		} else {
			secret = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name}}
		}

		if err = renew(secret, svc.Hosts(), now); err != nil {
			return err
		}
		if found {
			err = c.Update(ctx, secret)
		} else {
			err = c.Create(ctx, secret)
		}
		if err != nil {
			return err
		}
		klog.InfoS("Wrote serving cert secret", "secret", klog.KObj(secret), "created", !found)
		result = resultFor(secret, now)
		return nil
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to ensure serving cert secret %s/%s", namespace, name)
	}
	return result, nil
}

// renew puts a new serving cert into the Secret. If the CA in the Secret is
// usable, that CA signs the cert. If not, a new CA replaces it and signs the cert.
func renew(secret *corev1.Secret, hosts []string, now time.Time) error {
	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}
	oldCA := secret.Data[CAKey]
	data := secret.Data
	if caUsable(data, now) != nil {
		ca, err := generateCA(hosts[0], now.Add(-time.Hour), now.Add(Validity))
		if err != nil {
			return err
		}
		data = ca
		if len(certsIn(oldCA, now)) > 0 {
			// Keep trusting the old CA while servers switch to the new cert.
			secret.Data[PreviousCAKey] = oldCA
			setAnnotation(secret, PreviousCAUntilAnnotation, now.Add(CAOverlap).Format(time.RFC3339))
		} else {
			delete(secret.Data, PreviousCAKey)
			delete(secret.Annotations, PreviousCAUntilAnnotation)
		}
	}
	issued, err := issue(data, hosts, now.Add(-time.Hour), now.Add(ServingValidity))
	if err != nil {
		return err
	}
	for k, v := range issued {
		secret.Data[k] = v
	}
	return nil
}

func setAnnotation(secret *corev1.Secret, key, value string) {
	if secret.Annotations == nil {
		secret.Annotations = map[string]string{}
	}
	secret.Annotations[key] = value
}

func resultFor(secret *corev1.Secret, now time.Time) *Result {
	bundle := append([]byte{}, secret.Data[CAKey]...)
	if until, err := time.Parse(time.RFC3339, secret.Annotations[PreviousCAUntilAnnotation]); err == nil && now.Before(until) {
		if prev := secret.Data[PreviousCAKey]; len(certsIn(prev, now)) > 0 && !bytes.Equal(prev, secret.Data[CAKey]) {
			if len(bundle) > 0 && bundle[len(bundle)-1] != '\n' {
				bundle = append(bundle, '\n')
			}
			bundle = append(bundle, prev...)
		}
	}
	return &Result{Cert: secret.Data[CertKey], Key: secret.Data[KeyKey], CABundle: bundle}
}

// Check returns nil if data holds a key pair whose cert chains to the CA,
// covers every host and is valid at validUntil. If not, it returns the reason.
func Check(data map[string][]byte, hosts []string, validUntil time.Time) error {
	if _, err := tls.X509KeyPair(data[CertKey], data[KeyKey]); err != nil {
		return errors.Wrap(err, "invalid key pair")
	}
	roots, err := cert.NewPoolFromBytes(data[CAKey])
	if err != nil {
		return errors.Wrapf(err, "no CA certificate under key %q", CAKey)
	}
	block, _ := pem.Decode(data[CertKey])
	if block == nil {
		return errors.New("failed to decode certificate PEM")
	}
	serving, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return errors.Wrap(err, "invalid serving certificate")
	}
	for _, host := range hosts {
		opts := x509.VerifyOptions{
			Roots:       roots,
			CurrentTime: validUntil,
			KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		}
		if ip := net.ParseIP(host); ip != nil {
			opts.DNSName = ""
			found := false
			for _, certIP := range serving.IPAddresses {
				if certIP.Equal(ip) {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("serving certificate does not cover IP %s", host)
			}
		} else {
			opts.DNSName = host
		}
		if _, err = serving.Verify(opts); err != nil {
			return errors.Wrapf(err, "serving certificate not valid for %s at %s", host, validUntil.Format(time.RFC3339))
		}
	}
	return nil
}

// caUsable returns nil if data holds a CA cert and its matching key, and the
// CA is valid from now until RenewBefore after now.
func caUsable(data map[string][]byte, now time.Time) error {
	caCert, _, err := parseCA(data)
	if err != nil {
		return err
	}
	if now.Before(caCert.NotBefore) {
		return fmt.Errorf("CA is not valid before %s", caCert.NotBefore.Format(time.RFC3339))
	}
	if now.Add(RenewBefore).After(caCert.NotAfter) {
		return fmt.Errorf("CA expires at %s", caCert.NotAfter.Format(time.RFC3339))
	}
	return nil
}

func parseCA(data map[string][]byte) (*x509.Certificate, crypto.Signer, error) {
	pair, err := tls.X509KeyPair(data[CAKey], data[CAKeyKey])
	if err != nil {
		return nil, nil, errors.Wrap(err, "no usable CA key pair")
	}
	caCert, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return nil, nil, errors.Wrap(err, "invalid CA certificate")
	}
	signer, ok := pair.PrivateKey.(crypto.Signer)
	if !ok || !caCert.IsCA {
		return nil, nil, fmt.Errorf("CA under key %q cannot sign certificates", CAKey)
	}
	return caCert, signer, nil
}

// WriteFileIfChanged writes data to path, creating its directory, unless the
// file already holds it, and reports whether it wrote. The file is written atomically
// via a temporary file and rename to avoid reading partial files during reloads.
// The file is always left readable by its owner only (0600), also when it already
// existed with wider permissions, because it can hold a private key.
func WriteFileIfChanged(path string, data []byte) (bool, error) {
	cleanPath := filepath.Clean(path)
	if current, err := os.ReadFile(cleanPath); err == nil {
		if bytes.Equal(current, data) {
			if err := os.Chmod(cleanPath, 0o600); err != nil {
				return false, errors.Wrapf(err, "failed to set permissions of %s", path)
			}
			return false, nil
		}
	}
	dir := filepath.Dir(cleanPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return false, errors.Wrapf(err, "failed to create directory for %s", path)
	}

	tmpFile, err := os.CreateTemp(dir, "."+filepath.Base(cleanPath)+"-*.tmp")
	if err != nil {
		return false, errors.Wrapf(err, "failed to create temp file for %s", path)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if err := tmpFile.Chmod(0o600); err != nil {
		_ = tmpFile.Close()
		return false, errors.Wrapf(err, "failed to chmod temp file for %s", path)
	}
	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return false, errors.Wrapf(err, "failed to write temp file for %s", path)
	}
	if err := tmpFile.Close(); err != nil {
		return false, errors.Wrapf(err, "failed to close temp file for %s", path)
	}

	if err := os.Rename(tmpPath, cleanPath); err != nil {
		return false, errors.Wrapf(err, "failed to rename temp file to %s", path)
	}
	return true, nil
}

// Covers reports whether every certificate in bundle is also in trusted.
func Covers(trusted, bundle []byte) bool {
	if len(bytes.TrimSpace(bundle)) == 0 {
		return true
	}
	certs, err := cert.ParseCertsPEM(bundle)
	if err != nil {
		return false
	}
	have := map[string]bool{}
	if trustedCerts, err := cert.ParseCertsPEM(trusted); err == nil {
		for _, c := range trustedCerts {
			have[string(c.Raw)] = true
		}
	}
	for _, c := range certs {
		if !have[string(c.Raw)] {
			return false
		}
	}
	return true
}

func certsIn(bundle []byte, now time.Time) []*x509.Certificate {
	certs, err := cert.ParseCertsPEM(bundle)
	if err != nil {
		return nil
	}
	var valid []*x509.Certificate
	for _, c := range certs {
		if now.Before(c.NotAfter) {
			valid = append(valid, c)
		}
	}
	return valid
}

// Generate makes a self-signed CA and uses it to sign a serving cert for hosts.
// Both are valid from notBefore to notAfter. It returns them as Secret data.
func Generate(hosts []string, notBefore, notAfter time.Time) (map[string][]byte, error) {
	if len(hosts) == 0 {
		return nil, errors.New("no hosts to put in the serving cert")
	}
	ca, err := generateCA(hosts[0], notBefore, notAfter)
	if err != nil {
		return nil, err
	}
	return issue(ca, hosts, notBefore, notAfter)
}

func generateCA(name string, notBefore, notAfter time.Time) (map[string][]byte, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate CA key")
	}
	template := &x509.Certificate{
		Subject:               pkix.Name{CommonName: name + "-ca"},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	if template.SerialNumber, err = randomSerial(); err != nil {
		return nil, err
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create CA certificate")
	}
	keyPEM, err := keyutil.MarshalPrivateKeyToPEM(key)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal private key")
	}
	return map[string][]byte{
		CAKey:    pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		CAKeyKey: keyPEM,
	}, nil
}

func issue(ca map[string][]byte, hosts []string, notBefore, notAfter time.Time) (map[string][]byte, error) {
	caCert, caKey, err := parseCA(ca)
	if err != nil {
		return nil, err
	}
	if notAfter.After(caCert.NotAfter) {
		notAfter = caCert.NotAfter
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate serving key")
	}

	var dnsNames []string
	var ipAddresses []net.IP
	for _, host := range hosts {
		if ip := net.ParseIP(host); ip != nil {
			ipAddresses = append(ipAddresses, ip)
		} else {
			dnsNames = append(dnsNames, host)
		}
	}

	commonName := hosts[0]
	if len(dnsNames) > 0 {
		commonName = dnsNames[0]
	}

	template := &x509.Certificate{
		Subject:     pkix.Name{CommonName: commonName},
		DNSNames:    dnsNames,
		IPAddresses: ipAddresses,
		NotBefore:   notBefore,
		NotAfter:    notAfter,
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	if template.SerialNumber, err = randomSerial(); err != nil {
		return nil, err
	}
	der, err := x509.CreateCertificate(rand.Reader, template, caCert, &key.PublicKey, caKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create serving certificate")
	}
	keyPEM, err := keyutil.MarshalPrivateKeyToPEM(key)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal private key")
	}
	return map[string][]byte{
		CAKey:    ca[CAKey],
		CAKeyKey: ca[CAKeyKey],
		CertKey:  pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		KeyKey:   keyPEM,
	}, nil
}

func randomSerial() (*big.Int, error) {
	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate certificate serial number")
	}
	if serial.Sign() <= 0 {
		serial = big.NewInt(1)
	}
	return serial, nil
}
