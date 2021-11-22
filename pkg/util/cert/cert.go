package cert

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"

	"github.com/oam-dev/cluster-gateway/pkg/common"
	"github.com/openshift/library-go/pkg/crypto"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/cert"
)

var (
	rsaKeySize = 2048 // a decent number, as of 2019
)

func EnsureCAPair(cfg *rest.Config, namespace, name string) (*crypto.CA, error) {
	c, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	generate := false
	current, err := c.CoreV1().
		Secrets(namespace).
		Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
		generate = true
	}

	if !generate {
		caCertData := current.Data["ca.crt"]
		caKeyData := current.Data["ca.key"]
		certBlock, _ := pem.Decode(caCertData)
		caCert, err := x509.ParseCertificate(certBlock.Bytes)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse ca certificate")
		}
		keyBlock, _ := pem.Decode(caKeyData)
		caKey, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse ca key")
		}
		return &crypto.CA{
			Config: &crypto.TLSCertificateConfig{
				Certs: []*x509.Certificate{caCert},
				Key:   caKey,
			},
			SerialGenerator: &crypto.RandomSerialGenerator{},
		}, nil
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, rsaKeySize)
	if err != nil {
		return nil, err
	}
	caCert, err := cert.NewSelfSignedCACert(cert.Config{
		CommonName: common.AddonName,
	}, privateKey)
	if err != nil {
		return nil, err
	}
	rawKeyData, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, err
	}
	if _, err := c.CoreV1().
		Secrets(namespace).
		Create(context.TODO(), &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      name,
			},
			Data: map[string][]byte{
				"ca.crt": pem.EncodeToMemory(&pem.Block{
					Type:  "CERTIFICATE",
					Bytes: caCert.Raw,
				}),
				"ca.key": pem.EncodeToMemory(&pem.Block{
					Type:  "PRIVATE KEY",
					Bytes: rawKeyData,
				}),
			},
		}, metav1.CreateOptions{}); err != nil {
		return nil, err
	}
	return &crypto.CA{
		Config: &crypto.TLSCertificateConfig{
			Certs: []*x509.Certificate{caCert},
			Key:   privateKey,
		},
		SerialGenerator: &crypto.RandomSerialGenerator{},
	}, nil
}
