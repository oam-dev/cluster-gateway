package v1alpha1

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/klog"
	"sigs.k8s.io/apiserver-runtime/pkg/builder/resource"
	"sigs.k8s.io/apiserver-runtime/pkg/builder/resource/resourcerest"
	contextutil "sigs.k8s.io/apiserver-runtime/pkg/util/context"
)

var _ resource.SubResource = &ClusterExtensionFinalize{}
var _ resourcerest.Getter = &ClusterExtensionFinalize{}
var _ resourcerest.Updater = &ClusterExtensionFinalize{}

// ClusterExtensionFinalize
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ClusterExtensionFinalize struct {
	metav1.TypeMeta `json:",inline"`
}

func (c *ClusterExtensionFinalize) SubResourceName() string {
	return "finalize"
}

func (c *ClusterExtensionFinalize) New() runtime.Object {
	return &ClusterExtensionFinalize{}
}

func (c *ClusterExtensionFinalize) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	parentStorage, ok := contextutil.GetParentStorage(ctx)
	if !ok {
		return nil, fmt.Errorf("no parent storage found in the context")
	}
	klog.Infof("Getting finalize subresource of %v", name)
	return parentStorage.Get(ctx, name, options)
}

func (c *ClusterExtensionFinalize) Update(
	ctx context.Context,
	name string,
	objInfo rest.UpdatedObjectInfo,
	createValidation rest.ValidateObjectFunc,
	updateValidation rest.ValidateObjectUpdateFunc,
	forceAllowCreate bool,
	options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	parentStorage, ok := contextutil.GetParentStorage(ctx)
	if !ok {
		return nil, false, fmt.Errorf("no parent storage found in the context")
	}
	return parentStorage.Update(ctx, name, objInfo, createValidation, updateValidation, forceAllowCreate, options)
}
