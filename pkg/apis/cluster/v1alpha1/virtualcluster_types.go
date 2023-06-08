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
	"encoding/json"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
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

	// AnnotationClusterDescription the annotation key for cluster description
	AnnotationClusterDescription = config.MetaApiGroupName + "/cluster-description"

	// AnnotationClusterVersion the annotation key for cluster version
	AnnotationClusterVersion = config.MetaApiGroupName + "/cluster-version"

	// AnnotationClusterHealth the annotation key for cluster health
	AnnotationClusterHealth = config.MetaApiGroupName + "/cluster-health"

	// AnnotationClusterResources the annotation key for cluster resources
	AnnotationClusterResources = config.MetaApiGroupName + "/cluster-resources"

	// LabelClusterControlPlane identifies whether the cluster is the control plane
	LabelClusterControlPlane = config.MetaApiGroupName + "/control-plane"
)

// VirtualCluster is an extension model for cluster underlying secrets or OCM ManagedClusters
// +k8s:deepcopy-gen=false
type VirtualCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VirtualClusterSpec   `json:"spec,omitempty"`
	Status VirtualClusterStatus `json:"status,omitempty"`

	// Raw stores the underlying object
	Raw client.Object `json:"-"`
}

func (in *VirtualCluster) DeepCopy() *VirtualCluster {
	if in == nil {
		return nil
	}
	out := new(VirtualCluster)
	in.DeepCopyInto(out)
	return out
}

func (in *VirtualCluster) DeepCopyInto(out *VirtualCluster) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
	if in.Raw != nil {
		out.Raw = in.Raw.DeepCopyObject().(client.Object)
	} else {
		out.Raw = nil
	}
}

func (in *VirtualCluster) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// VirtualClusterSpec spec of cluster
type VirtualClusterSpec struct {
	Alias          string         `json:"alias,omitempty"`
	Description    string         `json:"description,omitempty"`
	Endpoint       string         `json:"endpoint,omitempty"`
	CredentialType CredentialType `json:"credentialType,omitempty"`
}

// VirtualClusterStatus status of cluster
type VirtualClusterStatus struct {
	// Health represents the cluster healthiness
	VirtualClusterHealthiness `json:",inline"`
	// Version represents the cluster version info of the managed cluster
	Version version.Info `json:"version,omitempty"`
	// Resources represents the cluster resources
	Resources VirtualClusterResources `json:"resources,omitempty"`
}

// VirtualClusterHealthiness the healthiness of the managed cluster
type VirtualClusterHealthiness struct {
	Healthy       bool        `json:"healthy,omitempty"`
	Reason        string      `json:"reason,omitempty"`
	LastProbeTime metav1.Time `json:"lastProbeTime,omitempty"`
}

// VirtualClusterResources the resources of the managed cluster
type VirtualClusterResources struct {
	// Capacity represents the total resource capacity from all nodeStatuses
	// on the managed cluster.
	Capacity corev1.ResourceList `json:"capacity,omitempty"`

	// Allocatable represents the total allocatable resources on the managed cluster.
	Allocatable corev1.ResourceList `json:"allocatable,omitempty"`

	// Usage represents the total resource usage on the managed cluster.
	Usage corev1.ResourceList `json:"usage,omitempty"`

	MasterNodeCount int `json:"masterNodeCount,omitempty"`
	WorkerNodeCount int `json:"workerNodeCount,omitempty"`
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
	return []string{"vcl", "vcls", "vcluster", "vclusters"}
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
		return obj.PrintTable(), nil
	case *VirtualClusterList:
		return obj.PrintTable(), nil
	default:
		return nil, fmt.Errorf("unprintable type %T", object)
	}
}

var (
	virtualClusterDefinitions = []metav1.TableColumnDefinition{
		{Name: "Name", Type: "string", Format: "name", Description: "the name of the cluster"},
		{Name: "Alias", Type: "string", Description: "the cluster provider type"},
		{Name: "Credential_Type", Type: "string", Description: "the credential type"},
		{Name: "Endpoint", Type: "string", Description: "the endpoint"},
		{Name: "Labels", Type: "string", Description: "the labels of the cluster", Priority: 20},
		{Name: "Version", Type: "string", Description: "the version of the cluster", Priority: 20},
		{Name: "Creation_Timestamp", Type: "dateTime", Description: "the creation timestamp of the cluster", Priority: 10},
	}
)

// PrintTable print table
func (in *VirtualCluster) PrintTable() *metav1.Table {
	return &metav1.Table{
		ColumnDefinitions: virtualClusterDefinitions,
		Rows:              []metav1.TableRow{in.printTableRow()},
	}
}

// PrintTable print table
func (in *VirtualClusterList) PrintTable() *metav1.Table {
	t := &metav1.Table{
		ColumnDefinitions: virtualClusterDefinitions,
	}
	for _, c := range in.Items {
		t.Rows = append(t.Rows, c.printTableRow())
	}
	return t
}

func (in *VirtualCluster) printTableRow() metav1.TableRow {
	var ls []string
	for k, v := range in.GetLabels() {
		ls = append(ls, fmt.Sprintf("%s=%s", k, v))
	}
	row := metav1.TableRow{
		Object: runtime.RawExtension{Object: in},
	}
	row.Cells = append(row.Cells,
		in.Name,
		in.Spec.Alias,
		in.Spec.CredentialType,
		in.Spec.Endpoint,
		strings.Join(ls, ","),
		in.Status.Version.String(),
		in.GetCreationTimestamp())
	return row
}

func dropMapKeyWithApiGroupPrefix(m map[string]string) map[string]string {
	_m := make(map[string]string)
	for k, v := range m {
		if !strings.HasPrefix(k, config.MetaApiGroupName) {
			_m[k] = v
		}
	}
	return _m
}

func newVirtualCluster(obj client.Object) *VirtualCluster {
	cluster := &VirtualCluster{Raw: obj}
	cluster.SetGroupVersionKind(SchemeGroupVersion.WithKind("VirtualCluster"))
	if obj != nil {
		cluster.SetName(obj.GetName())
		cluster.SetCreationTimestamp(obj.GetCreationTimestamp())
		cluster.SetLabels(dropMapKeyWithApiGroupPrefix(obj.GetLabels()))
		if annotations := obj.GetAnnotations(); annotations != nil {
			cluster.Spec.Alias = annotations[AnnotationClusterAlias]
			cluster.Spec.Description = annotations[AnnotationClusterDescription]
			_ = json.Unmarshal([]byte(annotations[AnnotationClusterVersion]), &cluster.Status.Version)
			_ = json.Unmarshal([]byte(annotations[AnnotationClusterHealth]), &cluster.Status.VirtualClusterHealthiness)
			if raw := annotations[AnnotationClusterResources]; raw != "" {
				_ = json.Unmarshal([]byte(annotations[AnnotationClusterResources]), &cluster.Status.Resources)
			}
		}
	}
	cluster.Spec.Endpoint = ClusterBlankEndpoint
	metav1.SetMetaDataLabel(&cluster.ObjectMeta, LabelClusterControlPlane, fmt.Sprintf("%t", obj == nil))
	return cluster
}

// NewLocalVirtualCluster return the local cluster
func NewLocalVirtualCluster() *VirtualCluster {
	cluster := newVirtualCluster(nil)
	cluster.SetName(ClusterLocalName)
	cluster.Spec.CredentialType = CredentialTypeInternal
	return cluster
}

// NewVirtualClusterFromSecret extract cluster from cluster secret
func NewVirtualClusterFromSecret(secret *corev1.Secret) (*VirtualCluster, error) {
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

// NewVirtualClusterFromManagedCluster extract cluster from ocm managed cluster
func NewVirtualClusterFromManagedCluster(managedCluster *ocmclusterv1.ManagedCluster) (*VirtualCluster, error) {
	if len(managedCluster.Spec.ManagedClusterClientConfigs) == 0 {
		return nil, NewInvalidManagedClusterError()
	}
	cluster := newVirtualCluster(managedCluster)
	cluster.Spec.CredentialType = CredentialTypeOCMManagedCluster
	return cluster, nil
}

// NewVirtualClusterFromObject create virtual cluster from existing object
func NewVirtualClusterFromObject(obj client.Object) (*VirtualCluster, error) {
	switch o := obj.(type) {
	case *corev1.Secret:
		return NewVirtualClusterFromSecret(o)
	case *ocmclusterv1.ManagedCluster:
		return NewVirtualClusterFromManagedCluster(o)
	default:
		return nil, fmt.Errorf("unrecognizable object type %T for virtual cluster", o)
	}
}
