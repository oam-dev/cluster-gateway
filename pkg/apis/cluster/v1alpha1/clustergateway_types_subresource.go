// +build !secret

package v1alpha1

import (
	"sigs.k8s.io/apiserver-runtime/pkg/builder/resource"
)

// ClusterGatewayStatus{} implements StatusSubResource interface.
var _ resource.StatusSubResource = &ClusterGatewayStatus{}

func (in ClusterGatewayStatus) SubResourceName() string {
	return "status"
}

func (in ClusterGatewayStatus) CopyTo(parent resource.ObjectWithStatusSubResource) {
	parent.(*ClusterGateway).Status = in
}

// ClusterGateway implements ObjectWithStatusSubResource interface.
var _ resource.ObjectWithStatusSubResource = &ClusterGateway{}

func (in *ClusterGateway) GetStatus() resource.StatusSubResource {
	return in.Status
}
