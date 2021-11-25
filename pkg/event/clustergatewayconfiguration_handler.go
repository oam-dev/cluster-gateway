package event

import (
	"context"

	proxyv1alpha1 "github.com/oam-dev/cluster-gateway/pkg/apis/proxy/v1alpha1"
	"github.com/oam-dev/cluster-gateway/pkg/common"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ handler.EventHandler = &ClusterGatewayConfigurationHandler{}

type ClusterGatewayConfigurationHandler struct {
	client.Client
}

func (c *ClusterGatewayConfigurationHandler) Create(event event.CreateEvent, q workqueue.RateLimitingInterface) {
	cfg := event.Object.(*proxyv1alpha1.ClusterGatewayConfiguration)
	c.process(cfg, q)
}

func (c *ClusterGatewayConfigurationHandler) Update(event event.UpdateEvent, q workqueue.RateLimitingInterface) {
	cfg := event.ObjectNew.(*proxyv1alpha1.ClusterGatewayConfiguration)
	c.process(cfg, q)
}

func (c *ClusterGatewayConfigurationHandler) Delete(event event.DeleteEvent, q workqueue.RateLimitingInterface) {
	cfg := event.Object.(*proxyv1alpha1.ClusterGatewayConfiguration)
	c.process(cfg, q)
}

func (c *ClusterGatewayConfigurationHandler) Generic(event event.GenericEvent, q workqueue.RateLimitingInterface) {
	cfg := event.Object.(*proxyv1alpha1.ClusterGatewayConfiguration)
	c.process(cfg, q)
}

func (c *ClusterGatewayConfigurationHandler) process(config *proxyv1alpha1.ClusterGatewayConfiguration, q workqueue.RateLimitingInterface) {
	list := addonv1alpha1.ClusterManagementAddOnList{}

	if err := c.Client.List(context.TODO(), &list); err != nil {
		ctrl.Log.WithName("ClusterGatewayConfiguration").Error(err, "failed list addons")
		return
	}

	for _, addon := range list.Items {
		if addon.Spec.AddOnConfiguration.CRDName != common.ClusterGatewayConfigurationCRDName {
			continue
		}
		if addon.Spec.AddOnConfiguration.CRName == config.Name {
			q.Add(reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: addon.Name,
				},
			})
		}
	}

}
