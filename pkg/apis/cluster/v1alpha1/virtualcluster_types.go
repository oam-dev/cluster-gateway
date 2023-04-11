/*
Copyright 2023 The KubeVela Authors.

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
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ocmclusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/apiserver-runtime/pkg/builder/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/oam-dev/cluster-gateway/pkg/common"
	"github.com/oam-dev/cluster-gateway/pkg/config"
	"github.com/oam-dev/cluster-gateway/pkg/util/singleton"
)

const (
	// ClusterLocalName name for the hub cluster
	ClusterLocalName = "local"
	// CredentialTypeInternal identifies the cluster from internal kubevela system
	CredentialTypeInternal CredentialType = "Internal"
	// CredentialTypeOCMManagedCluster identifies the ocm cluster
	CredentialTypeOCMManagedCluster CredentialType = "ManagedCluster"
	// ClusterBlankEndpoint identifies the endpoint of a cluster as blank (not available)
	ClusterBlankEndpoint = "-"
)

var (
	// AnnotationClusterAlias the annotation key for cluster alias
	AnnotationClusterAlias = config.MetaApiGroupName + "/cluster-alias"

	// LabelClusterControlPlane identifies whether the cluster is the control plane
	LabelClusterControlPlane = config.MetaApiGroupName + "/control-plane"
)

// VirtualCluster is an extension model for cluster underlying secrets or OCM ManagedClusters
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type VirtualCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec VirtualClusterSpec `json:"spec,omitempty"`
}

// VirtualClusterSpec spec of cluster
type VirtualClusterSpec struct {
	Alias          string         `json:"alias,omitempty"`
	Accepted       bool           `json:"accepted,omitempty"`
	Endpoint       string         `json:"endpoint,omitempty"`
	CredentialType CredentialType `json:"credential-type,omitempty"`
}

// VirtualClusterList list for VirtualCluster
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type VirtualClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []VirtualCluster `json:"items"`
}

var _ resource.Object = &VirtualCluster{}

// GetObjectMeta returns the object meta reference.
func (in *VirtualCluster) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

// NamespaceScoped returns if the object must be in a namespace.
func (in *VirtualCluster) NamespaceScoped() bool {
	return false
}

// New returns a new instance of the resource
func (in *VirtualCluster) New() runtime.Object {
	return &VirtualCluster{}
}

// Destroy .
func (in *VirtualCluster) Destroy() {}

// NewList return a new list instance of the resource
func (in *VirtualCluster) NewList() runtime.Object {
	return &VirtualClusterList{}
}

// GetGroupVersionResource returns the GroupVersionResource for this resource.
func (in *VirtualCluster) GetGroupVersionResource() schema.GroupVersionResource {
	return SchemeGroupVersion.WithResource("virtualclusters")
}

// IsStorageVersion returns true if the object is also the internal version
func (in *VirtualCluster) IsStorageVersion() bool {
	return true
}

// ShortNames delivers a list of short names for a resource.
func (in *VirtualCluster) ShortNames() []string {
	return []string{"vc", "vcluster", "vclusters", "virtual-cluster", "virtual-clusters"}
}

// GetFullName returns the name with alias
func (in *VirtualCluster) GetFullName() string {
	if in.Spec.Alias == "" {
		return in.GetName()
	}
	return fmt.Sprintf("%s (%s)", in.GetName(), in.Spec.Alias)
}

// HasCluster return if the cluster list contains a cluster with the specified name
func (in *VirtualClusterList) HasCluster(name string) bool {
	for _, cluster := range in.Items {
		if cluster.Name == name {
			return true
		}
	}
	return false
}

// Get finds a resource in the storage by name and returns it.
func (in *VirtualCluster) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	return NewVirtualClusterClient(singleton.GetCtrlClient(), config.SecretNamespace, config.VirtualClusterWithControlPlane).Get(ctx, name)
}

// List selects resources in the storage which match to the selector. 'options' can be nil.
func (in *VirtualCluster) List(ctx context.Context, options *internalversion.ListOptions) (runtime.Object, error) {
	sel := labels.NewSelector()
	if options != nil && options.LabelSelector != nil && !options.LabelSelector.Empty() {
		sel = options.LabelSelector
	}
	return NewVirtualClusterClient(singleton.GetCtrlClient(), config.SecretNamespace, config.VirtualClusterWithControlPlane).List(ctx, client.MatchingLabelsSelector{Selector: sel})
}

// ConvertToTable convert resource to table
func (in *VirtualCluster) ConvertToTable(ctx context.Context, object runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {
	switch obj := object.(type) {
	case *VirtualCluster:
		return printCluster(obj), nil
	case *VirtualClusterList:
		return printClusterList(obj), nil
	default:
		return nil, fmt.Errorf("unknown type %T", object)
	}
}

var (
	virtualClusterDefinitions = []metav1.TableColumnDefinition{
		{Name: "Name", Type: "string", Format: "name", Description: "the name of the cluster"},
		{Name: "Alias", Type: "string", Description: "the cluster provider type"},
		{Name: "Credential_Type", Type: "string", Description: "the credential type"},
		{Name: "Endpoint", Type: "string", Description: "the endpoint"},
		{Name: "Accepted", Type: "boolean", Description: "the acceptance of the cluster"},
		{Name: "Labels", Type: "string", Description: "the labels of the cluster"},
		{Name: "Creation_Timestamp", Type: "dateTime", Description: "the creation timestamp of the cluster", Priority: 10},
	}
)

func printCluster(in *VirtualCluster) *metav1.Table {
	return &metav1.Table{
		ColumnDefinitions: virtualClusterDefinitions,
		Rows:              []metav1.TableRow{printClusterRow(in)},
	}
}

func printClusterList(in *VirtualClusterList) *metav1.Table {
	t := &metav1.Table{
		ColumnDefinitions: virtualClusterDefinitions,
	}
	for _, c := range in.Items {
		t.Rows = append(t.Rows, printClusterRow(c.DeepCopy()))
	}
	return t
}

func printClusterRow(c *VirtualCluster) metav1.TableRow {
	var ls []string
	for k, v := range c.GetLabels() {
		ls = append(ls, fmt.Sprintf("%s=%s", k, v))
	}
	row := metav1.TableRow{
		Object: runtime.RawExtension{Object: c},
	}
	row.Cells = append(row.Cells,
		c.Name,
		c.Spec.Alias,
		c.Spec.CredentialType,
		c.Spec.Endpoint,
		c.Spec.Accepted,
		strings.Join(ls, ","),
		c.GetCreationTimestamp())
	return row
}

func extractLabels(labels map[string]string) map[string]string {
	_labels := make(map[string]string)
	for k, v := range labels {
		if !strings.HasPrefix(k, config.MetaApiGroupName) {
			_labels[k] = v
		}
	}
	return _labels
}

func newVirtualCluster(obj client.Object) *VirtualCluster {
	cluster := &VirtualCluster{}
	cluster.SetGroupVersionKind(SchemeGroupVersion.WithKind("VirtualCluster"))
	if obj != nil {
		cluster.SetName(obj.GetName())
		cluster.SetCreationTimestamp(obj.GetCreationTimestamp())
		cluster.SetLabels(extractLabels(obj.GetLabels()))
		if annotations := obj.GetAnnotations(); annotations != nil {
			cluster.Spec.Alias = annotations[AnnotationClusterAlias]
		}
	}
	cluster.Spec.Accepted = true
	cluster.Spec.Endpoint = ClusterBlankEndpoint
	metav1.SetMetaDataLabel(&cluster.ObjectMeta, LabelClusterControlPlane, fmt.Sprintf("%t", obj == nil))
	return cluster
}

// NewLocalCluster return the local cluster
func NewLocalCluster() *VirtualCluster {
	cluster := newVirtualCluster(nil)
	cluster.SetName(ClusterLocalName)
	cluster.Spec.CredentialType = CredentialTypeInternal
	return cluster
}

// NewClusterFromSecret extract cluster from cluster secret
func NewClusterFromSecret(secret *corev1.Secret) (*VirtualCluster, error) {
	cluster := newVirtualCluster(secret)
	cluster.Spec.Endpoint = string(secret.Data["endpoint"])
	if metav1.HasLabel(secret.ObjectMeta, common.LabelKeyClusterEndpointType) {
		cluster.Spec.Endpoint = secret.GetLabels()[common.LabelKeyClusterEndpointType]
	}
	if cluster.Spec.Endpoint == "" {
		return nil, NewEmptyEndpointClusterSecretError()
	}
	if !metav1.HasLabel(secret.ObjectMeta, common.LabelKeyClusterCredentialType) {
		return nil, NewEmptyCredentialTypeClusterSecretError()
	}
	cluster.Spec.CredentialType = CredentialType(secret.GetLabels()[common.LabelKeyClusterCredentialType])
	return cluster, nil
}

// NewClusterFromManagedCluster extract cluster from ocm managed cluster
func NewClusterFromManagedCluster(managedCluster *ocmclusterv1.ManagedCluster) (*VirtualCluster, error) {
	if len(managedCluster.Spec.ManagedClusterClientConfigs) == 0 {
		return nil, NewInvalidManagedClusterError()
	}
	cluster := newVirtualCluster(managedCluster)
	cluster.Spec.Accepted = managedCluster.Spec.HubAcceptsClient
	cluster.Spec.CredentialType = CredentialTypeOCMManagedCluster
	return cluster, nil
}
