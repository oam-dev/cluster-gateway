package event

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	proxyv1alpha1 "github.com/oam-dev/cluster-gateway/pkg/apis/proxy/v1alpha1"
	"github.com/oam-dev/cluster-gateway/pkg/common"
)

var _ handler.EventHandler = &ClusterGatewayConfigurationHandler{}

type ClusterGatewayConfigurationHandler struct {
	client.Client
}

func (c *ClusterGatewayConfigurationHandler) Create(ctx context.Context, event event.TypedCreateEvent[client.Object], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	cfg := event.Object.(*proxyv1alpha1.ClusterGatewayConfiguration)
	c.process(ctx, cfg, q)
}

func (c *ClusterGatewayConfigurationHandler) Update(ctx context.Context, event event.TypedUpdateEvent[client.Object], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	cfg := event.ObjectNew.(*proxyv1alpha1.ClusterGatewayConfiguration)
	c.process(ctx, cfg, q)
}

func (c *ClusterGatewayConfigurationHandler) Delete(ctx context.Context, event event.TypedDeleteEvent[client.Object], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	cfg := event.Object.(*proxyv1alpha1.ClusterGatewayConfiguration)
	c.process(ctx, cfg, q)
}

func (c *ClusterGatewayConfigurationHandler) Generic(ctx context.Context, event event.TypedGenericEvent[client.Object], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	cfg := event.Object.(*proxyv1alpha1.ClusterGatewayConfiguration)
	c.process(ctx, cfg, q)
}

func (c *ClusterGatewayConfigurationHandler) process(ctx context.Context, config *proxyv1alpha1.ClusterGatewayConfiguration, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	list := addonv1alpha1.ClusterManagementAddOnList{}

	if err := c.Client.List(ctx, &list); err != nil {
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
