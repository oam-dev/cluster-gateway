package v1alpha1

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8snet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/rest"
	"k8s.io/utils/pointer"
)

func TestClusterRestConfigConversion(t *testing.T) {
	testToken := "test-token"
	testCAData := []byte(`test-ca`)
	testCertData := []byte(`test-cert`)
	testKeyData := []byte(`test-key`)
	proxyURLData := []byte(`socks5://localhost:1080`)
	proxyURL, _ := url.Parse(string(proxyURLData))
	testDialFunc := func(ctx context.Context, net, addr string) (net.Conn, error) {
		return nil, nil
	}
	DialerGetter = func(ctx context.Context) (k8snet.DialFunc, error) {
		return testDialFunc, nil
	}
	cases := []struct {
		name           string
		clusterGateway *ClusterGateway
		expectedCfg    *rest.Config
		expectFailure  bool
	}{
		{
			name: "normal cluster-gateway with SA token + host-port should work",
			clusterGateway: &ClusterGateway{
				Spec: ClusterGatewaySpec{
					Access: ClusterAccess{
						Endpoint: &ClusterEndpoint{
							Type: ClusterEndpointTypeConst,
							Const: &ClusterEndpointConst{
								Address:  "https://foo.bar:33",
								CABundle: testCAData,
							},
						},
						Credential: &ClusterAccessCredential{
							Type:                CredentialTypeServiceAccountToken,
							ServiceAccountToken: testToken,
						},
					},
				},
			},
			expectedCfg: &rest.Config{
				Host:        "https://foo.bar:33",
				BearerToken: testToken,
				Timeout:     40 * time.Second,
				TLSClientConfig: rest.TLSClientConfig{
					ServerName: "foo.bar",
					CAData:     testCAData,
				},
			},
		},
		{
			name: "normal cluster-gateway with X509 + host-port should work",
			clusterGateway: &ClusterGateway{
				Spec: ClusterGatewaySpec{
					Access: ClusterAccess{
						Endpoint: &ClusterEndpoint{
							Type: ClusterEndpointTypeConst,
							Const: &ClusterEndpointConst{
								Address:  "https://foo.bar:33",
								CABundle: testCAData,
							},
						},
						Credential: &ClusterAccessCredential{
							Type: CredentialTypeX509Certificate,
							X509: &X509{
								Certificate: testCertData,
								PrivateKey:  testKeyData,
							},
						},
					},
				},
			},
			expectedCfg: &rest.Config{
				Host:    "https://foo.bar:33",
				Timeout: 40 * time.Second,
				TLSClientConfig: rest.TLSClientConfig{
					ServerName: "foo.bar",
					CAData:     testCAData,
					CertData:   testCertData,
					KeyData:    testKeyData,
				},
			},
		},
		{
			name: "https port defaulting should work",
			clusterGateway: &ClusterGateway{
				Spec: ClusterGatewaySpec{
					Access: ClusterAccess{
						Endpoint: &ClusterEndpoint{
							Type: ClusterEndpointTypeConst,
							Const: &ClusterEndpointConst{
								Address:  "https://foo.bar",
								CABundle: testCAData,
							},
						},
						Credential: &ClusterAccessCredential{
							Type:                CredentialTypeServiceAccountToken,
							ServiceAccountToken: testToken,
						},
					},
				},
			},
			expectedCfg: &rest.Config{
				Host:        "https://foo.bar",
				Timeout:     40 * time.Second,
				BearerToken: testToken,
				TLSClientConfig: rest.TLSClientConfig{
					ServerName: "foo.bar",
					CAData:     testCAData,
				},
			},
		},
		{
			name: "insecure (no CA bundle) should work",
			clusterGateway: &ClusterGateway{
				Spec: ClusterGatewaySpec{
					Access: ClusterAccess{
						Endpoint: &ClusterEndpoint{
							Type: ClusterEndpointTypeConst,
							Const: &ClusterEndpointConst{
								Address:  "https://foo.bar:33",
								Insecure: pointer.Bool(true),
							},
						},
						Credential: &ClusterAccessCredential{
							Type:                CredentialTypeServiceAccountToken,
							ServiceAccountToken: testToken,
						},
					},
				},
			},
			expectedCfg: &rest.Config{
				Host:        "https://foo.bar:33",
				Timeout:     40 * time.Second,
				BearerToken: testToken,
				TLSClientConfig: rest.TLSClientConfig{
					ServerName: "foo.bar",
					Insecure:   true,
				},
			},
		},
		{
			name: "cluster-proxy egress should work",
			clusterGateway: &ClusterGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-cluster",
				},
				Spec: ClusterGatewaySpec{
					Access: ClusterAccess{
						Endpoint: &ClusterEndpoint{
							Type: ClusterEndpointTypeClusterProxy,
						},
						Credential: &ClusterAccessCredential{
							Type:                CredentialTypeServiceAccountToken,
							ServiceAccountToken: testToken,
						},
					},
				},
			},
			expectedCfg: &rest.Config{
				Host:        "my-cluster",
				Timeout:     40 * time.Second,
				BearerToken: testToken,
				Dial:        testDialFunc,
				TLSClientConfig: rest.TLSClientConfig{
					Insecure: true,
				},
			},
		},
		{
			name: "proxy-url should work",
			clusterGateway: &ClusterGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-cluster",
				},
				Spec: ClusterGatewaySpec{
					Access: ClusterAccess{
						Endpoint: &ClusterEndpoint{
							Type: ClusterEndpointTypeConst,
							Const: &ClusterEndpointConst{
								Address:  "https://foo.bar:33",
								ProxyURL: pointer.String(string(proxyURLData)),
							},
						},
						Credential: &ClusterAccessCredential{
							Type:                CredentialTypeServiceAccountToken,
							ServiceAccountToken: testToken,
						},
					},
				},
			},
			expectedCfg: &rest.Config{
				Host:        "https://foo.bar:33",
				Timeout:     40 * time.Second,
				BearerToken: testToken,
				Proxy:       http.ProxyURL(proxyURL),
				TLSClientConfig: rest.TLSClientConfig{
					ServerName: "foo.bar",
				},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg, err := NewConfigFromCluster(context.TODO(), c.clusterGateway)
			if err != nil {
				if c.expectFailure {
					return
				}
				require.NoError(t, err)
			}
			if cfg.Dial != nil {
				assert.ObjectsAreEqual(c.expectedCfg.Dial, cfg.Dial)
				c.expectedCfg.Dial = nil
				cfg.Dial = nil
			}
			if cfg.Proxy != nil {
				assert.NotNil(t, c.expectedCfg.Proxy)
				cfg.Proxy = nil
				c.expectedCfg.Proxy = nil
			}
			assert.Equal(t, c.expectedCfg, cfg)
		})
	}

}
