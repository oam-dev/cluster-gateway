// +build !secret

package v1alpha1

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/apiserver-runtime/pkg/builder/resource/resourcestrategy"
)

var _ resourcestrategy.TableConverter = &ClusterGateway{}
var _ resourcestrategy.TableConverter = &ClusterGatewayList{}

func (in *ClusterGateway) ConvertToTable(ctx context.Context, tableOptions runtime.Object) (*metav1.Table, error) {
	return printClusterGateway(in), nil
}

func (in *ClusterGatewayList) ConvertToTable(ctx context.Context, tableOptions runtime.Object) (*metav1.Table, error) {
	return printClusterGatewayList(in), nil
}
