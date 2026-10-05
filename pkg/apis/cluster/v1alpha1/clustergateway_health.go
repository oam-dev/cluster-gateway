package v1alpha1

import (
	"context"
	"fmt"
	"strconv"

	"github.com/oam-dev/cluster-gateway/pkg/config"
	"github.com/oam-dev/cluster-gateway/pkg/util/singleton"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"
)

var _ rest.Getter = &ClusterGatewayHealth{}
var _ rest.Updater = &ClusterGatewayHealth{}

// ClusterGatewayHealth is a subresource for ClusterGateway which allows
// updating and reading the health status of the managed cluster. It is
// never itself serialized on the wire: Get resolves through Parent storage,
// while Update persists health annotations into the backing Secret and
// returns the converted *ClusterGateway from that updated Secret.
//
// +k8s:openapi-gen=false
type ClusterGatewayHealth struct {
	// Parent is the storage of the parent ClusterGateway resource, used to
	// resolve the target cluster for the health subresource.
	Parent rest.Getter
}

func (in *ClusterGatewayHealth) New() runtime.Object {
	return &ClusterGateway{}
}

func (in *ClusterGatewayHealth) SubResourceName() string {
	return "health"
}

func (in *ClusterGatewayHealth) Destroy() {}

func (in *ClusterGatewayHealth) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	if in.Parent == nil {
		return nil, fmt.Errorf("no parent storage found")
	}
	parentObj, err := in.Parent.Get(ctx, name, options)
	if err != nil {
		return nil, fmt.Errorf("no such cluster %v", name)
	}
	clusterGateway := parentObj.(*ClusterGateway)
	return clusterGateway, nil
}

func (in *ClusterGatewayHealth) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo, createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc, forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	if singleton.GetSecretControl() == nil {
		return nil, false, fmt.Errorf("loopback clients are not inited")
	}

	latestSecret, err := singleton.GetSecretControl().Get(ctx, name)
	if err != nil {
		return nil, false, err
	}
	updating, err := objInfo.UpdatedObject(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	updatingClusterGateway := updating.(*ClusterGateway)
	if latestSecret.Annotations == nil {
		latestSecret.Annotations = make(map[string]string)
	}
	latestSecret.Annotations[AnnotationKeyClusterGatewayStatusHealthy] = strconv.FormatBool(updatingClusterGateway.Status.Healthy)
	latestSecret.Annotations[AnnotationKeyClusterGatewayStatusHealthyReason] = string(updatingClusterGateway.Status.HealthyReason)
	updated, err := singleton.GetKubeClient().
		CoreV1().
		Secrets(config.SecretNamespace).
		Update(ctx, latestSecret, metav1.UpdateOptions{})
	if err != nil {
		return nil, false, err
	}
	clusterGateway, err := convertFromSecret(updated)
	if err != nil {
		return nil, false, err
	}
	return clusterGateway, false, nil
}
