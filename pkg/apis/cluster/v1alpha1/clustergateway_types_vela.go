// +build vela

package v1alpha1

import (
	"context"
	"fmt"

	"github.com/oam-dev/cluster-gateway/pkg/config"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/apiserver-runtime/pkg/util/loopback"
)

var _ rest.Getter = &ClusterGateway{}

func (in *ClusterGateway) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	masterLoopbackConfig := loopback.GetLoopbackMasterClientConfig()
	c, err := kubernetes.NewForConfig(masterLoopbackConfig)
	if err != nil {
		return nil, err
	}
	clusterSecret, err := c.CoreV1().Secrets(config.SecretNamespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return convert(clusterSecret)
}

func convert(secret *v1.Secret) (*ClusterGateway, error) {
	endpoint, ok := secret.Data["endpoint"]
	if !ok {
		return nil, fmt.Errorf("invalid secret: missing key %q", "endpoint")
	}
	c := &ClusterGateway{
		Spec: ClusterGatewaySpec{
			Provider: "",
			Access: ClusterAccess{
				Endpoint: string(endpoint),
			},
		},
	}

	if ca, ok := secret.Data["ca.crt"]; ok {
		c.Spec.Access.CABundle = ca
	} else {
		c.Spec.Access.Insecure = pointer.Bool(true)
	}
	switch secret.Type {
	case v1.SecretTypeTLS:
		c.Spec.Access.Credential = &ClusterAccessCredential{
			Type: CredentialTypeX509Certificate,
			X509: &X509{
				Certificate: secret.Data[v1.TLSCertKey],
				PrivateKey:  secret.Data[v1.TLSPrivateKeyKey],
			},
		}
	case v1.SecretTypeServiceAccountToken:
		c.Spec.Access.Credential = &ClusterAccessCredential{
			Type:                CredentialTypeServiceAccountToken,
			ServiceAccountToken: string(secret.Data[v1.ServiceAccountTokenKey]),
		}
	}
	return c, nil
}
