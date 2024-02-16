package v1alpha1

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/client-go/pkg/apis/clientauthentication"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/utils/pointer"

	"github.com/oam-dev/cluster-gateway/pkg/common"
	"github.com/oam-dev/cluster-gateway/pkg/config"
	"github.com/oam-dev/cluster-gateway/pkg/featuregates"
	"github.com/oam-dev/cluster-gateway/pkg/options"
	"github.com/oam-dev/cluster-gateway/pkg/util/exec"
	"github.com/oam-dev/cluster-gateway/pkg/util/singleton"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/klog/v2"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
)

var _ rest.Getter = &ClusterGateway{}
var _ rest.Lister = &ClusterGateway{}

// Conversion between corev1.Secret and ClusterGateway:
//  1. Storing credentials under the secret's data including X.509 key-pair or token.
//  2. Extending the spec of ClusterGateway by the secret's label.
//  3. Extending the status of ClusterGateway by the secrets' annotation.
//
// NOTE: Because the secret resource is designed to have no "metadata.generation" field,
// the ClusterGateway resource also misses the generation tracking.
const (
	AnnotationKeyClusterGatewayStatusHealthy       = "status.cluster.core.oam.dev/healthy"
	AnnotationKeyClusterGatewayStatusHealthyReason = "status.cluster.core.oam.dev/healthy-reason"
)

func (in *ClusterGateway) Get(ctx context.Context, name string, _ *metav1.GetOptions) (runtime.Object, error) {
	if singleton.GetSecretControl() == nil {
		return nil, fmt.Errorf("loopback secret client are not inited")
	}

	clusterSecret, err := singleton.GetSecretControl().Get(ctx, name)
	if err != nil {
		klog.Warningf("Failed getting secret %q/%q: %v", config.SecretNamespace, name, err)
		return nil, err
	}

	if options.OCMIntegration {
		if singleton.GetClusterControl() == nil {
			return nil, fmt.Errorf("loopback cluster client are not inited")
		}
		managedCluster, err := singleton.GetClusterControl().Get(ctx, name)
		if err != nil {
			return convertFromSecret(clusterSecret)
		}
		return convertFromManagedClusterAndSecret(managedCluster, clusterSecret)
	}

	return convertFromSecret(clusterSecret)
}

func (in *ClusterGateway) List(ctx context.Context, opt *internalversion.ListOptions) (runtime.Object, error) {
	if opt.Watch {
		// TODO: convert watch events from both Secret and ManagedCluster
		return nil, fmt.Errorf("watch not supported")
	}
	clusterSecrets, err := singleton.GetSecretControl().List(ctx)
	if err != nil {
		return nil, err
	}
	list := &ClusterGatewayList{
		Items: []ClusterGateway{},
	}

	if options.OCMIntegration {
		clusters, err := singleton.GetClusterControl().List(ctx)
		if err != nil {
			return nil, err
		}
		clustersByName := make(map[string]*clusterv1.ManagedCluster)
		for _, cluster := range clusters {
			clustersByName[cluster.Name] = cluster
		}

		for _, secret := range clusterSecrets {
			if cluster, ok := clustersByName[secret.Name]; ok {
				gw, err := convertFromManagedClusterAndSecret(cluster, secret)
				if err != nil {
					klog.Warningf("skipping %v: failed converting clustergateway resource", secret.Name)
					continue
				}
				list.Items = append(list.Items, *gw)
			} else {
				gw, err := convertFromSecret(secret)
				if err != nil {
					klog.Warningf("skipping %v: failed converting clustergateway resource", secret.Name)
					continue
				}
				list.Items = append(list.Items, *gw)
			}
		}
		return list, nil
	}

	for _, secret := range clusterSecrets {
		gw, err := convertFromSecret(secret)
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
		return nil, "", nil
	}
	cfg := managedCluster.Spec.ManagedClusterClientConfigs[0]
	return cfg.CABundle, cfg.URL, nil
}

func getEndpointFromSecret(secret *v1.Secret) ([]byte, string, error) {
	endpoint := secret.Data["endpoint"]
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
			Name:              secret.Name,
			CreationTimestamp: secret.CreationTimestamp,
		},
		Spec: ClusterGatewaySpec{
			Access: ClusterAccess{},
		},
	}

	// converting endpoint
	endpointType, ok := secret.Labels[common.LabelKeyClusterEndpointType]
	if !ok {
		endpointType = string(ClusterEndpointTypeConst)
	}
	var proxyURL *string
	if url, useProxy := secret.Data["proxy-url"]; useProxy && len(url) > 0 {
		proxyURL = pointer.String(string(url))
	}
	switch ClusterEndpointType(endpointType) {
	case ClusterEndpointTypeClusterProxy:
		c.Spec.Access.Endpoint = &ClusterEndpoint{
			Type: ClusterEndpointType(endpointType),
		}
	case ClusterEndpointTypeConst:
		fallthrough // backward compatibility
	default:
		if len(apiServerEndpoint) == 0 {
			return nil, errors.New("missing label key: api-endpoint")
		}
		if insecure {
			c.Spec.Access.Endpoint = &ClusterEndpoint{
				Type: ClusterEndpointType(endpointType),
				Const: &ClusterEndpointConst{
					Address:  apiServerEndpoint,
					Insecure: &insecure,
					ProxyURL: proxyURL,
				},
			}
		} else {
			c.Spec.Access.Endpoint = &ClusterEndpoint{
				Type: ClusterEndpointType(endpointType),
				Const: &ClusterEndpointConst{
					Address:  apiServerEndpoint,
					CABundle: caData,
					ProxyURL: proxyURL,
				},
			}
		}
	}

	// converting credential
	credentialType, ok := secret.Labels[common.LabelKeyClusterCredentialType]
	if !ok {
		return nil, apierrors.NewNotFound(schema.GroupResource{
			Group:    config.MetaApiGroupName,
			Resource: config.MetaApiResourceName,
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

	case CredentialTypeDynamic:
		credential, err := buildCredentialFromExecConfig(secret, c.Spec.Access.Endpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to issue credential from external command: %s", err)
		}

		c.Spec.Access.Credential = credential

	default:
		return nil, fmt.Errorf("unrecognized secret credential type %v", credentialType)
	}

	if utilfeature.DefaultMutableFeatureGate.Enabled(featuregates.HealthinessCheck) {
		if healthyRaw, ok := secret.Annotations[AnnotationKeyClusterGatewayStatusHealthy]; ok {
			healthy, err := strconv.ParseBool(healthyRaw)
			if err != nil {
				return nil, fmt.Errorf("unrecogized healthiness status: %v", healthyRaw)
			}
			c.Status.Healthy = healthy
		}
		if healthyReason, ok := secret.Annotations[AnnotationKeyClusterGatewayStatusHealthyReason]; ok {
			c.Status.HealthyReason = HealthyReasonType(healthyReason)
		}
	}

	if utilfeature.DefaultMutableFeatureGate.Enabled(featuregates.ClientIdentityPenetration) {
		if proxyConfigRaw, ok := secret.Annotations[AnnotationClusterGatewayProxyConfiguration]; ok {
			proxyConfig := &ClusterGatewayProxyConfiguration{}
			if err := yaml.Unmarshal([]byte(proxyConfigRaw), proxyConfig); err == nil {
				for _, rule := range proxyConfig.Spec.Rules {
					rule.Source.Cluster = pointer.String(c.Name)
				}
				c.Spec.ProxyConfig = proxyConfig
			}
		}
	}

	return c, nil
}

func buildCredentialFromExecConfig(secret *v1.Secret, ep *ClusterEndpoint) (*ClusterAccessCredential, error) {
	execConfigRaw := secret.Data["exec"]
	if len(execConfigRaw) == 0 {
		return nil, errors.New("missing secret data key: exec")
	}

	var ec clientcmdapi.ExecConfig
	if err := json.Unmarshal(execConfigRaw, &ec); err != nil {
		return nil, fmt.Errorf("failed to decode exec config JSON from secret data: %v", err)
	}

	cluster := &clientauthentication.Cluster{
		Server:                   ep.Const.Address,
		CertificateAuthorityData: ep.Const.CABundle,
		InsecureSkipTLSVerify:    pointer.BoolDeref(ep.Const.Insecure, false),
		ProxyURL:                 pointer.StringDeref(ep.Const.ProxyURL, ""),
	}

	cred, err := exec.GetToken(&ec, cluster)
	if err != nil {
		return nil, err
	}

	if token := cred.Status.Token; len(token) > 0 {
		return &ClusterAccessCredential{
			Type:                CredentialTypeServiceAccountToken,
			ServiceAccountToken: token,
		}, nil
	}

	if cert, key := cred.Status.ClientCertificateData, cred.Status.ClientKeyData; len(cert) > 0 && len(key) > 0 {
		return &ClusterAccessCredential{
			Type: CredentialTypeX509Certificate,
			X509: &X509{
				Certificate: []byte(cert),
				PrivateKey:  []byte(key),
			},
		}, nil

	}

	return nil, fmt.Errorf("no credential type available")
}
