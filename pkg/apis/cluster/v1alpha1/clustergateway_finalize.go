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

var _ resource.SubResource = &ClusterGatewayFinalize{}
var _ resourcerest.Getter = &ClusterGatewayFinalize{}
var _ resourcerest.Updater = &ClusterGatewayFinalize{}

// ClusterGatewayFinalize
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ClusterGatewayFinalize struct {
	metav1.TypeMeta `json:",inline"`
}

func (c *ClusterGatewayFinalize) SubResourceName() string {
	return "finalize"
}

func (c *ClusterGatewayFinalize) New() runtime.Object {
	return &ClusterGatewayFinalize{}
}

func (c *ClusterGatewayFinalize) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	parentStorage, ok := contextutil.GetParentStorage(ctx)
	if !ok {
		return nil, fmt.Errorf("no parent storage found in the context")
	}
	klog.Infof("Getting finalize subresource of %v", name)
	return parentStorage.Get(ctx, name, options)
}

func (c *ClusterGatewayFinalize) Update(
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
