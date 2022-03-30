package common

import "github.com/oam-dev/cluster-gateway/pkg/config"

const (
	AddonName = "cluster-gateway"
)

const (
	LabelKeyOpenClusterManagementAddon = "proxy.open-cluster-management.io/addon-name"
)

const (
	ClusterGatewayConfigurationCRDName = "clustergatewayconfigurations.proxy.open-cluster-management.io"
	ClusterGatewayConfigurationCRName  = "cluster-gateway"
)

const (
	InstallNamespace = "open-cluster-management-cluster-gateway"
)

const (
	ClusterGatewayAPIServiceName = "v1alpha1.cluster.core.oam.dev"
)

var (
	// LabelKeyClusterCredentialType describes the credential type in object label field
	LabelKeyClusterCredentialType = config.MetaApiGroupName + "/cluster-credential-type"
	// LabelKeyClusterEndpointType describes the endpoint type.
	LabelKeyClusterEndpointType = config.MetaApiGroupName + "/cluster-endpoint-type"
)
