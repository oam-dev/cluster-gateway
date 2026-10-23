package servingcert

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newTestScheme(t *testing.T) *runtime.Scheme {
	s := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(s))
	return s
}

func TestGenerateAndCheck(t *testing.T) {
	now := time.Now()
	hosts := []string{"gateway-service", "gateway-service.default.svc", "127.0.0.1"}

	data, err := Generate(hosts, now.Add(-time.Hour), now.Add(ServingValidity))
	require.NoError(t, err)
	require.NotEmpty(t, data[CAKey])
	require.NotEmpty(t, data[CAKeyKey])
	require.NotEmpty(t, data[CertKey])
	require.NotEmpty(t, data[KeyKey])

	// Check should succeed at current time
	err = Check(data, hosts, now)
	require.NoError(t, err)

	// Check should fail for a host not covered
	err = Check(data, []string{"other-service.default.svc"}, now)
	require.Error(t, err)

	// Check should fail if past expiration
	err = Check(data, hosts, now.Add(ServingValidity+time.Hour))
	require.Error(t, err)
}

func TestEnsureSecretCreateAndReuse(t *testing.T) {
	scheme := newTestScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	ctx := context.Background()

	svc := Service{Namespace: "default", Name: "gateway-service"}
	res, err := EnsureSecret(ctx, c, "default", "gateway-tls", svc)
	require.NoError(t, err)
	require.NotEmpty(t, res.Cert)
	require.NotEmpty(t, res.Key)
	require.NotEmpty(t, res.CABundle)

	// Secret must exist in client
	secret := &corev1.Secret{}
	err = c.Get(ctx, types.NamespacedName{Namespace: "default", Name: "gateway-tls"}, secret)
	require.NoError(t, err)
	require.Equal(t, res.Cert, secret.Data[CertKey])
	require.Equal(t, res.Key, secret.Data[KeyKey])

	// Calling EnsureSecret again should reuse the exact same cert and key
	res2, err := EnsureSecret(ctx, c, "default", "gateway-tls", svc)
	require.NoError(t, err)
	require.Equal(t, res.Cert, res2.Cert)
	require.Equal(t, res.Key, res2.Key)
	require.Equal(t, res.CABundle, res2.CABundle)
}

func TestEnsureSecretRenewalReusesCA(t *testing.T) {
	scheme := newTestScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	ctx := context.Background()

	svc := Service{Namespace: "default", Name: "gateway-service"}
	_, err := EnsureSecret(ctx, c, "default", "gateway-tls", svc)
	require.NoError(t, err)

	secret := &corev1.Secret{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Namespace: "default", Name: "gateway-tls"}, secret))
	originalCA := secret.Data[CAKey]

	// Simulate serving cert expiring soon by rewriting it with a short-lived cert
	now := time.Now()
	expiredServing, err := issue(secret.Data, svc.Hosts(), now.Add(-2*time.Hour), now.Add(10*time.Minute))
	require.NoError(t, err)
	secret.Data[CertKey] = expiredServing[CertKey]
	secret.Data[KeyKey] = expiredServing[KeyKey]
	require.NoError(t, c.Update(ctx, secret))

	// EnsureSecret should detect near-expiry and renew serving cert, but KEEP the CA
	resRenewed, err := EnsureSecret(ctx, c, "default", "gateway-tls", svc)
	require.NoError(t, err)
	require.Equal(t, originalCA, resRenewed.CABundle)
	require.NotEqual(t, expiredServing[CertKey], resRenewed.Cert)

	// Verify that the new cert chains to the original CA
	roots := x509.NewCertPool()
	require.True(t, roots.AppendCertsFromPEM(originalCA))
	block, _ := pem.Decode(resRenewed.Cert)
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)
	_, err = cert.Verify(x509.VerifyOptions{
		Roots:       roots,
		DNSName:     svc.RequiredHosts()[0],
		CurrentTime: now,
	})
	require.NoError(t, err)
}

func TestEnsureSecretCARotationWithOverlap(t *testing.T) {
	scheme := newTestScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	ctx := context.Background()

	svc := Service{Namespace: "default", Name: "gateway-service"}
	// Create an initial secret with a CA that is valid now but expiring soon (within RenewBefore)
	now := time.Now()
	oldData, err := Generate(svc.Hosts(), now.Add(-20*time.Hour), now.Add(2*time.Hour))
	require.NoError(t, err)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "gateway-tls"},
		Data:       oldData,
	}
	require.NoError(t, c.Create(ctx, secret))

	// EnsureSecret should rotate the CA
	res, err := EnsureSecret(ctx, c, "default", "gateway-tls", svc)
	require.NoError(t, err)

	require.NoError(t, c.Get(ctx, types.NamespacedName{Namespace: "default", Name: "gateway-tls"}, secret))
	require.NotEqual(t, oldData[CAKey], secret.Data[CAKey])
	require.NotEmpty(t, secret.Annotations[PreviousCAUntilAnnotation])
	require.Equal(t, oldData[CAKey], secret.Data[PreviousCAKey])

	// CABundle should contain both the new CA and the previous CA during overlap
	require.True(t, bytes.Contains(res.CABundle, secret.Data[CAKey]))
	require.True(t, bytes.Contains(res.CABundle, oldData[CAKey]))
}

func TestWriteFileIfChanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tls.key")
	data1 := []byte("private-key-1")
	data2 := []byte("private-key-2")

	// First write creates file
	written, err := WriteFileIfChanged(path, data1)
	require.NoError(t, err)
	require.True(t, written)

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	// Identical write does not modify
	written, err = WriteFileIfChanged(path, data1)
	require.NoError(t, err)
	require.False(t, written)

	// Modified write updates
	written, err = WriteFileIfChanged(path, data2)
	require.NoError(t, err)
	require.True(t, written)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, data2, content)
}

func TestCovers(t *testing.T) {
	now := time.Now()
	hosts := []string{"gateway-service"}
	data1, err := Generate(hosts, now.Add(-time.Hour), now.Add(Validity))
	require.NoError(t, err)

	data2, err := Generate(hosts, now.Add(-time.Hour), now.Add(Validity))
	require.NoError(t, err)

	require.True(t, Covers(data1[CAKey], data1[CAKey]))
	require.False(t, Covers(data1[CAKey], data2[CAKey]))
	require.True(t, Covers(data1[CAKey], nil))
}
