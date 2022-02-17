package v1alpha1

import (
	"context"
	"net"
	"net/url"
	"strconv"

	"github.com/oam-dev/cluster-gateway/pkg/config"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	grpccredentials "google.golang.org/grpc/credentials"
	k8snet "k8s.io/apimachinery/pkg/util/net"
	restclient "k8s.io/client-go/rest"
	konnectivity "sigs.k8s.io/apiserver-network-proxy/konnectivity-client/pkg/client"
	"sigs.k8s.io/apiserver-network-proxy/pkg/util"
)

var DialerGetter = func() (k8snet.DialFunc, error) {
	tlsCfg, err := util.GetClientTLSConfig(
		config.ClusterProxyCAFile,
		config.ClusterProxyCertFile,
		config.ClusterProxyKeyFile,
		config.ClusterProxyHost,
		nil)
	if err != nil {
		return nil, err
	}
	tunnel, err := konnectivity.CreateSingleUseGrpcTunnel(
		context.TODO(),
		net.JoinHostPort(config.ClusterProxyHost, strconv.Itoa(config.ClusterProxyPort)),
		grpc.WithTransportCredentials(grpccredentials.NewTLS(tlsCfg)),
	)
	if err != nil {
		return nil, err
	}
	return tunnel.DialContext, nil
}

func NewConfigFromCluster(c *ClusterGateway) (*restclient.Config, error) {
	cfg := &restclient.Config{}
	// setting up endpoint
	switch c.Spec.Access.Endpoint.Type {
	case ClusterEndpointTypeConst:
		cfg.Host = c.Spec.Access.Endpoint.Const.Address
		cfg.CAData = c.Spec.Access.Endpoint.Const.CABundle
		if c.Spec.Access.Endpoint.Const.Insecure != nil && *c.Spec.Access.Endpoint.Const.Insecure {
			cfg.TLSClientConfig = restclient.TLSClientConfig{Insecure: true}
		}
		u, err := url.Parse(c.Spec.Access.Endpoint.Const.Address)
		if err != nil {
			return nil, err
		}

		const missingPort = "missing port in address"
		host, _, err := net.SplitHostPort(u.Host)
		if err != nil {
			addrErr, ok := err.(*net.AddrError)
			if !ok {
				return nil, err
			}
			if addrErr.Err != missingPort {
				return nil, err
			}
			host = u.Host
		}
		cfg.ServerName = host // apiserver may listen on SNI cert
	case ClusterEndpointTypeClusterProxy:
		cfg.Host = c.Name // the same as the cluster name
		cfg.Insecure = true
		cfg.CAData = nil
		dail, err := DialerGetter()
		if err != nil {
			return nil, err
		}
		cfg.Dial = dail
	}
	// setting up credentials
	switch c.Spec.Access.Credential.Type {
	case CredentialTypeServiceAccountToken:
		cfg.BearerToken = c.Spec.Access.Credential.ServiceAccountToken
	case CredentialTypeX509Certificate:
		cfg.CertData = c.Spec.Access.Credential.X509.Certificate
		cfg.KeyData = c.Spec.Access.Credential.X509.PrivateKey
	}
	return cfg, nil
}

func GetEndpointURL(c *ClusterGateway) (*url.URL, error) {
	switch c.Spec.Access.Endpoint.Type {
	case ClusterEndpointTypeConst:
		urlAddr, err := url.Parse(c.Spec.Access.Endpoint.Const.Address)
		if err != nil {
			return nil, errors.Wrapf(err, "failed parsing url from cluster %s invalid value %s",
				c.Name, c.Spec.Access.Endpoint.Const.Address)
		}
		return urlAddr, nil
	case ClusterEndpointTypeClusterProxy:
		return &url.URL{
			Scheme: "https",
			Host:   c.Name,
		}, nil
	default:
		return nil, errors.New("unsupported cluster gateway endpoint type")
	}
}
