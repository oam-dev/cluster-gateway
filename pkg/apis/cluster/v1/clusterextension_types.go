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

package v1

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/apiserver-runtime/pkg/builder/resource"
	"sigs.k8s.io/apiserver-runtime/pkg/builder/resource/resourcestrategy"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
//
// ClusterExtension is an extension model for ManagedCluster which implements
// the Tier-II cluster model based on OCM's original abstraction of
// ManagedCluster. The Tier-II cluster model should be highly protected under
// RBAC policies and only the admin shall have the access to view the content
// of cluster credentials.
//
// Documentation: https://yuque.antfin.com/antcloud-paas/ar858o/tku0n9#6433b698
// +k8s:openapi-gen=true
type ClusterExtension struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterExtensionSpec   `json:"spec,omitempty"`
	Status ClusterExtensionStatus `json:"status,omitempty"`
}

// ClusterExtensionList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ClusterExtensionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ClusterExtension `json:"items"`
}

// ClusterExtensionSpec defines the desired state of ClusterExtension
type ClusterExtensionSpec struct {
	Provider  string        `json:"provider"`
	Access    ClusterAccess `json:"access"`
	Finalized *bool         `json:"finalized,omitempty"`
}

type ClusterAccess struct {
	// Endpoint is a qualified URL string for accessing the cluster.
	// e.g. https://example.com:6443/
	Endpoint string `json:"endpoint"`
	// CABundle is used for verifying cluster's serving CA certificate.
	CABundle []byte `json:"caBundle,omitempty"`
	// Insecure indicates the cluster should be access'd w/o verifying
	// CA certificate at client-side.
	Insecure   *bool                    `json:"insecure,omitempty"`
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

var _ resource.Object = &ClusterExtension{}
var _ resourcestrategy.Validater = &ClusterExtension{}

func (in *ClusterExtension) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

func (in *ClusterExtension) NamespaceScoped() bool {
	return false
}

func (in *ClusterExtension) New() runtime.Object {
	return &ClusterExtension{}
}

func (in *ClusterExtension) NewList() runtime.Object {
	return &ClusterExtensionList{}
}

func (in *ClusterExtension) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "core.oam.dev",
		Version:  "v1",
		Resource: "clusterextensions",
	}
}

func (in *ClusterExtension) IsStorageVersion() bool {
	return true
}

func (in *ClusterExtension) Validate(ctx context.Context) field.ErrorList {
	return ValidateClusterExtension(in)
}

var _ resource.ObjectList = &ClusterExtensionList{}

func (in *ClusterExtensionList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// ClusterExtensionStatus defines the observed state of ClusterExtension
type ClusterExtensionStatus struct {
	Healthy bool `json:"healthy,omitempty"`
}

func (in ClusterExtensionStatus) SubResourceName() string {
	return "status"
}

// ClusterExtension implements ObjectWithStatusSubResource interface.
var _ resource.ObjectWithStatusSubResource = &ClusterExtension{}

func (in *ClusterExtension) GetStatus() resource.StatusSubResource {
	return in.Status
}

// ClusterExtensionStatus{} implements StatusSubResource interface.
var _ resource.StatusSubResource = &ClusterExtensionStatus{}

func (in ClusterExtensionStatus) CopyTo(parent resource.ObjectWithStatusSubResource) {
	parent.(*ClusterExtension).Status = in
}

var _ resource.ObjectWithArbitrarySubResource = &ClusterExtension{}

func (in *ClusterExtension) GetArbitrarySubResources() []resource.ArbitrarySubResource {
	return []resource.ArbitrarySubResource{
		&ClusterExtensionProxy{},
		&ClusterExtensionFinalize{},
	}
}
