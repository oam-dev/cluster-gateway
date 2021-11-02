/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"context"

	"github.com/oam-dev/cluster-gateway/pkg/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/apiserver-runtime/pkg/builder/resource"
	"sigs.k8s.io/apiserver-runtime/pkg/builder/resource/resourcestrategy"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
//
// ClusterGateway is an extension model for ManagedCluster which implements
// the Tier-II cluster model based on OCM's original abstraction of
// ManagedCluster. The Tier-II cluster model should be highly protected under
// RBAC policies and only the admin shall have the access to view the content
// of cluster credentials.
// +k8s:openapi-gen=true
type ClusterGateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterGatewaySpec   `json:"spec,omitempty"`
	Status ClusterGatewayStatus `json:"status,omitempty"`
}

// ClusterGatewayList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ClusterGatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ClusterGateway `json:"items"`
}

// ClusterGatewaySpec defines the desired state of ClusterGateway
type ClusterGatewaySpec struct {
	Provider string        `json:"provider"`
	Access   ClusterAccess `json:"access"`
}

type ClusterAccess struct {
	// Endpoint is a qualified URL string for accessing the cluster.
	// e.g. https://example.com:6443/
	Endpoint *ClusterEndpoint `json:"endpoint"`
	// Credential holds authentication configuration for
	// accessing the target cluster.
	Credential *ClusterAccessCredential `json:"credential,omitempty"`
}

type CredentialType string

const (
	// CredentialTypeServiceAccountToken means the cluster is accessible via
	// ServiceAccountToken.
	CredentialTypeServiceAccountToken CredentialType = "ServiceAccountToken"
	// CredentialTypeX509Certificate means the cluster is accessible via
	// X509 certificate and key.
	CredentialTypeX509Certificate CredentialType = "X509Certificate"
)

var (
	// LabelKeyClusterCredentialType describes the credential type in object label field
	LabelKeyClusterCredentialType = config.MetaApiGroupName + "/cluster-credential-type"
	// LabelKeyClusterEndpointType describes the endpoint type.
	LabelKeyClusterEndpointType = config.MetaApiGroupName + "/cluster-endpoint-type"
)

type ClusterEndpointType string

type ClusterEndpoint struct {
	// Type is the union discriminator for prescribing apiserver endpoint.
	Type ClusterEndpointType `json:"type"`
	// Const prescribes fixed endpoint for requesting target clusters.
	Const *ClusterEndpointConst `json:"const,omitempty"`
}

const (
	// ClusterEndpointTypeConst prescribes requesting kube-apiserver via
	// a fixed endpoint url.
	ClusterEndpointTypeConst = "Const"
	// ClusterEndpointTypeClusterProxy prescribes requesting kube-apiserver
	// through the konnectivity tunnel. Note that no explicit endpoint are
	// required under ClusterProxy mode.
	ClusterEndpointTypeClusterProxy = "ClusterProxy"
)

type ClusterEndpointConst struct {
	// Address is a qualified hostname for accessing the local kube-apiserver.
	Address string `json:"address"`
	// CABundle is used for verifying cluster's serving CA certificate.
	CABundle []byte `json:"caBundle,omitempty"`
	// Insecure indicates the cluster should be access'd w/o verifying
	// CA certificate at client-side.
	Insecure *bool `json:"insecure,omitempty"`
}

type ClusterAccessCredential struct {
	// Type is the union discriminator for credential contents.
	Type                CredentialType `json:"type"`
	ServiceAccountToken string         `json:"serviceAccountToken,omitempty"`
	X509                *X509          `json:"x509,omitempty"`
}

type X509 struct {
	Certificate []byte `json:"certificate"`
	PrivateKey  []byte `json:"privateKey"`
}

var _ resource.Object = &ClusterGateway{}
var _ resourcestrategy.Validater = &ClusterGateway{}

func (in *ClusterGateway) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

func (in *ClusterGateway) NamespaceScoped() bool {
	return false
}

func (in *ClusterGateway) New() runtime.Object {
	return &ClusterGateway{}
}

func (in *ClusterGateway) NewList() runtime.Object {
	return &ClusterGatewayList{}
}

func (in *ClusterGateway) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    config.MetaApiGroupName,
		Version:  config.MetaApiVersionName,
		Resource: config.MetaApiResourceName,
	}
}

func (in *ClusterGateway) IsStorageVersion() bool {
	return true
}

func (in *ClusterGateway) Validate(ctx context.Context) field.ErrorList {
	return ValidateClusterGateway(in)
}

var _ resource.ObjectList = &ClusterGatewayList{}

func (in *ClusterGatewayList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// ClusterGatewayStatus defines the observed state of ClusterGateway
type ClusterGatewayStatus struct {
	Healthy bool `json:"healthy,omitempty"`
}

var _ resource.ObjectWithArbitrarySubResource = &ClusterGateway{}

func (in *ClusterGateway) GetArbitrarySubResources() []resource.ArbitrarySubResource {
	return []resource.ArbitrarySubResource{
		&ClusterGatewayProxy{},
	}
}
