// +build vela

package v1alpha1

import (
	"context"
	"fmt"

	"github.com/oam-dev/cluster-gateway/pkg/config"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/apiserver-runtime/pkg/util/loopback"
)

const (
	// Label keys
	LabelKeyClusterCredentialType = "cluster.core.oam.dev/cluster-credential-type"
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
		ObjectMeta: metav1.ObjectMeta{
			Name: secret.Name,
		},
		Spec: ClusterGatewaySpec{
			Provider: "",
			Access: ClusterAccess{
				Endpoint: string(endpoint),
			},
		},
	}
	credentialType, ok := secret.Labels[LabelKeyClusterCredentialType]
	if !ok {
		return nil, errors.NewNotFound(schema.GroupResource{
			Group:    "cluster.core.oam.dev",
			Resource: "clustergateways",
		}, secret.Name)
	}

	if caCrt, ok := secret.Data["ca.crt"]; ok {
		c.Spec.Access.CABundle = caCrt
	} else if ca, ok := secret.Data["ca"]; ok {
		c.Spec.Access.CABundle = ca
	} else {
		c.Spec.Access.Insecure = pointer.Bool(true)
	}
	switch CredentialType(credentialType) {
	case CredentialTypeX509Certificate:
		c.Spec.Access.Credential = &ClusterAccessCredential{
			Type: CredentialTypeX509Certificate,
			X509: &X509{
				Certificate: secret.Data[v1.TLSCertKey],
				PrivateKey:  secret.Data[v1.TLSPrivateKeyKey],
			},
		}
	case CredentialTypeServiceAccountToken:
		c.Spec.Access.Credential = &ClusterAccessCredential{
			Type:                CredentialTypeServiceAccountToken,
			ServiceAccountToken: string(secret.Data[v1.ServiceAccountTokenKey]),
		}
	default:
		return nil, fmt.Errorf("unrecognized secret type %v", credentialType)
	}
	return c, nil
}
