package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

func init() {
	SchemeBuilder.Register(&ClusterGatewayConfiguration{}, &ClusterGatewayConfigurationList{})
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ClusterGatewayConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterGatewayConfigurationSpec   `json:"spec,omitempty"`
	Status ClusterGatewayConfigurationStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ClusterGatewayConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterGatewayConfiguration `json:"items"`
}

type ClusterGatewayConfigurationSpec struct {
	// +required
	Image string `json:"image"`
	// +required
	SecretNamespace string `json:"secretNamespace"`
	// +required
	InstallNamespace string `json:"installNamespace"`
	// +required
	SecretManagement ClusterGatewaySecretManagement `json:"secretManagement"`
	// +required
	Egress ClusterGatewayTrafficEgress `json:"egress"`
}

type ClusterGatewayConfigurationStatus struct {
	// +optional
	LastObservedGeneration int64 `json:"lastObservedGeneration,omitempty"`
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type ClusterGatewayTrafficEgress struct {
	Type         EgressType                               `json:"type"`
	ClusterProxy *ClusterGatewayTrafficEgressClusterProxy `json:"clusterProxy,omitempty"`
}

type EgressType string

const (
	EgressTypeDirect       = "Direct"
	EgressTypeClusterProxy = "ClusterProxy"
)

type ClusterGatewayTrafficEgressClusterProxy struct {
	ProxyServerHost string                                            `json:"proxyServerHost"`
	ProxyServerPort int32                                             `json:"proxyServerPort"`
	Credentials     ClusterGatewayTrafficEgressClusterProxyCredential `json:"credentials"`
}

type ClusterGatewayTrafficEgressClusterProxyCredential struct {
	Namespace               string `json:"namespace"`
	ProxyClientSecretName   string `json:"proxyClientSecretName"`
	ProxyClientCASecretName string `json:"proxyClientCASecretName"`
}

type ClusterGatewaySecretManagement struct {
	// +optional
	// +kubebuilder:default=ManagedServiceAccount
	Type SecretManagementType `json:"type"`
	// +optional
	ManagedServiceAccount *SecretManagementManagedServiceAccount `json:"managedServiceAccount,omitempty"`
}

// +kubebuilder:validation:Enum=Manual;ManagedServiceAccount
type SecretManagementType string

const (
	SecretManagementTypeManual                = "Manual"
	SecretManagementTypeManagedServiceAccount = "ManagedServiceAccount"
)

type SecretManagementManagedServiceAccount struct {
	// +optional
	// +kubebuilder:default=cluster-gateway
	Name string `json:"name"`
}

const (
	ConditionTypeClusterGatewayDeployed = "ClusterGatewayDeployed"
)
