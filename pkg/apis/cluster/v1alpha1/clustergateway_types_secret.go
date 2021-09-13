// +build secret

package v1alpha1

import (
	"context"
	"fmt"
	"strings"

	"github.com/oam-dev/cluster-gateway/pkg/config"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
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
var _ rest.Lister = &ClusterGateway{}

func (in *ClusterGateway) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	c, err := kubernetes.NewForConfig(loopback.GetLoopbackMasterClientConfig())
	if err != nil {
		return nil, err
	}
	clusterSecret, err := c.CoreV1().Secrets(config.SecretNamespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return convert(clusterSecret)
}

func (in *ClusterGateway) List(ctx context.Context, options *internalversion.ListOptions) (runtime.Object, error) {
	if options.Watch {
		return nil, fmt.Errorf("watch not supported")
	}
	c, err := kubernetes.NewForConfig(loopback.GetLoopbackMasterClientConfig())
	if err != nil {
		return nil, err
	}
	requirement, err := labels.NewRequirement(
		LabelKeyClusterCredentialType,
		selection.Exists,
		nil)
	if err != nil {
		return nil, err
	}
	clusterSecrets, err := c.CoreV1().Secrets(config.SecretNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.NewSelector().
			Add(*requirement).
			String(),
	})
	if err != nil {
		return nil, err
	}
	list := &ClusterGatewayList{
		Items: []ClusterGateway{},
	}
	for _, secret := range clusterSecrets.Items {
		gw, err := convert(&secret)
		if err != nil {
			return nil, err
		}
		list.Items = append(list.Items, *gw)
	}
	return list, nil
}

func (in *ClusterGateway) ConvertToTable(ctx context.Context, object runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {
	switch object.(type) {
	case *ClusterGateway:
		return printClusterGateway(object.(*ClusterGateway)), nil
	case *ClusterGatewayList:
		return printClusterGatewayList(object.(*ClusterGatewayList)), nil
	default:
		return nil, fmt.Errorf("unknown type %T", object)
	}
}

func convert(secret *v1.Secret) (*ClusterGateway, error) {
	endpoint, ok := secret.Data["endpoint"]
	if !ok {
		return nil, fmt.Errorf("invalid secret: missing key %q", "endpoint")
	}
	endpointStr := string(endpoint)
	endpointStr = strings.TrimSuffix(endpointStr, "\n")
	c := &ClusterGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name: secret.Name,
		},
		Spec: ClusterGatewaySpec{
			Provider: "",
			Access: ClusterAccess{
				Endpoint: endpointStr,
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
