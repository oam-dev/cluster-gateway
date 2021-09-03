package v1alpha1

import (
	"net"
	"net/url"

	"github.com/pkg/errors"

	restclient "k8s.io/client-go/rest"
)

func NewConfigFromCluster(c *ClusterGateway) (*restclient.Config, error) {
	cfg := &restclient.Config{}
	cfg.Host = c.Spec.Access.Endpoint
	cfg.CAData = c.Spec.Access.CABundle
	if c.Spec.Access.Insecure != nil && *c.Spec.Access.Insecure {
		cfg.TLSClientConfig = restclient.TLSClientConfig{Insecure: true}
	}
	switch c.Spec.Access.Credential.Type {
	case CredentialTypeServiceAccountToken:
		cfg.BearerToken = c.Spec.Access.Credential.ServiceAccountToken
	case CredentialTypeX509Certificate:
		cfg.CertData = c.Spec.Access.Credential.X509.Certificate
		cfg.KeyData = c.Spec.Access.Credential.X509.PrivateKey
	}
	u, err := url.Parse(c.Spec.Access.Endpoint)
	if err != nil {
		return nil, err
	}
	host, _, err := net.SplitHostPort(u.Host)
	if err != nil {
		return nil, err
	}
	cfg.ServerName = host // apiserver may listen on SNI cert
	return cfg, nil
}

func GetEndpointURL(c *ClusterGateway) (*url.URL, error) {
	urlAddr, err := url.Parse(c.Spec.Access.Endpoint)
	if err != nil {
		return nil, errors.Wrapf(err, "failed parsing url from cluster %s invalid value %s", c.Name, c.Spec.Access.Endpoint)
	}
	return urlAddr, nil
}
