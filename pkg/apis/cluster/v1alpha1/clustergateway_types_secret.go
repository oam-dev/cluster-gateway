// +build secret

package v1alpha1

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/oam-dev/cluster-gateway/pkg/config"
	"github.com/oam-dev/cluster-gateway/pkg/options"

	ocmclient "open-cluster-management.io/api/client/cluster/clientset/versioned"
	clusterv1 "open-cluster-management.io/api/cluster/v1"

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
	"k8s.io/klog/v2"
	"sigs.k8s.io/apiserver-runtime/pkg/util/loopback"
)

var _ rest.Getter = &ClusterGateway{}
var _ rest.Lister = &ClusterGateway{}

var initClient sync.Once
var kubeClient kubernetes.Interface
var ocmClient ocmclient.Interface

func (in *ClusterGateway) Get(ctx context.Context, name string, _ *metav1.GetOptions) (runtime.Object, error) {
	initClientOnce()
	clusterSecret, err := kubeClient.
		CoreV1().
		Secrets(config.SecretNamespace).
		Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if options.OCMIntegration {
		managedCluster, err := ocmClient.
			ClusterV1().
			ManagedClusters().
			Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return convertFromManagedClusterAndSecret(managedCluster, clusterSecret)
	}

	return convertFromSecret(clusterSecret)
}

func (in *ClusterGateway) List(ctx context.Context, opt *internalversion.ListOptions) (runtime.Object, error) {
	if opt.Watch {
		return nil, fmt.Errorf("watch not supported")
	}
	initClientOnce()
	requirement, err := labels.NewRequirement(
		LabelKeyClusterCredentialType,
		selection.Exists,
		nil)
	if err != nil {
		return nil, err
	}

	clusterSecrets, err := kubeClient.
		CoreV1().
		Secrets(config.SecretNamespace).
		List(ctx, metav1.ListOptions{
			LabelSelector: labels.NewSelector().Add(*requirement).String(),
		})
	if err != nil {
		return nil, err
	}
	list := &ClusterGatewayList{
		Items: []ClusterGateway{},
	}

	if options.OCMIntegration {
		clusters, err := ocmClient.
			ClusterV1().
			ManagedClusters().
			List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		clustersByName := make(map[string]*clusterv1.ManagedCluster)
		for _, cluster := range clusters.Items {
			cluster := cluster
			clustersByName[cluster.Name] = &cluster
		}
		for _, secret := range clusterSecrets.Items {
			if cluster, ok := clustersByName[secret.Name]; ok {
				gw, err := convertFromManagedClusterAndSecret(cluster, &secret)
				if err != nil {
					klog.Warningf("skipping %v: failed converting clustergateway resource",
						secret.Name)
					continue
				}
				list.Items = append(list.Items, *gw)
			}
		}
		return list, nil
	}

	for _, secret := range clusterSecrets.Items {
		gw, err := convertFromSecret(&secret)
		if err != nil {
			klog.Errorf("failed converting secret to gateway: %v", err)
			continue
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

func initClientOnce() {
	initClient.Do(func() {
		kubeClient = kubernetes.NewForConfigOrDie(loopback.GetLoopbackMasterClientConfig())
		ocmClient = ocmclient.NewForConfigOrDie(loopback.GetLoopbackMasterClientConfig())
	})
}

func convertFromSecret(clusterSecret *v1.Secret) (*ClusterGateway, error) {
	caData, endpoint, err := getEndpointFromSecret(clusterSecret)
	if err != nil {
		return nil, err
	}
	return convert(caData, endpoint, caData == nil, clusterSecret)
}

func convertFromManagedClusterAndSecret(managedCluster *clusterv1.ManagedCluster, clusterSecret *v1.Secret) (*ClusterGateway, error) {
	caData, endpoint, err := getEndpointFromManagedCluster(managedCluster)
	if err != nil {
		return nil, err
	}
	return convert(caData, endpoint, false, clusterSecret)
}

func getEndpointFromManagedCluster(managedCluster *clusterv1.ManagedCluster) ([]byte, string, error) {
	if len(managedCluster.Spec.ManagedClusterClientConfigs) == 0 {
		return nil, "", fmt.Errorf("no external endpoint configured for cluster %v", managedCluster.Name)
	}
	cfg := managedCluster.Spec.ManagedClusterClientConfigs[0]
	return cfg.CABundle, cfg.URL, nil
}

func getEndpointFromSecret(secret *v1.Secret) ([]byte, string, error) {
	endpoint, ok := secret.Data["endpoint"]
	if !ok {
		return nil, "", fmt.Errorf("invalid secret: missing key %q", "endpoint")
	}
	endpointStr := string(endpoint)
	endpointStr = strings.TrimSuffix(endpointStr, "\n")

	var caData []byte = nil
	if caCrt, ok := secret.Data["ca.crt"]; ok {
		caData = caCrt
	} else if ca, ok := secret.Data["ca"]; ok {
		caData = ca
	} else {
		caData = nil
	}
	return caData, endpointStr, nil
}

func convert(caData []byte, apiServerEndpoint string, insecure bool, secret *v1.Secret) (*ClusterGateway, error) {
	c := &ClusterGateway{
		ObjectMeta: metav1.ObjectMeta{
			Name: secret.Name,
		},
		Spec: ClusterGatewaySpec{
			Provider: "",
			Access: ClusterAccess{
				CABundle: caData,
				Insecure: &insecure,
				Endpoint: apiServerEndpoint,
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
