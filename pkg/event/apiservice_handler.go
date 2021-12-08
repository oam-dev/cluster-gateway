package event

import (
	"github.com/oam-dev/cluster-gateway/pkg/common"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ handler.EventHandler = &APIServiceHandler{}

const (
	clusterGatewayAPIServiceName = "v1alpha1.cluster.core.oam.dev"
)

type APIServiceHandler struct {
}

func (a *APIServiceHandler) Create(event event.CreateEvent, q workqueue.RateLimitingInterface) {
	a.process(event.Object.(*apiregistrationv1.APIService), q)
}

func (a *APIServiceHandler) Update(event event.UpdateEvent, q workqueue.RateLimitingInterface) {
	a.process(event.ObjectNew.(*apiregistrationv1.APIService), q)
}

func (a *APIServiceHandler) Delete(event event.DeleteEvent, q workqueue.RateLimitingInterface) {
	a.process(event.Object.(*apiregistrationv1.APIService), q)
}

func (a *APIServiceHandler) Generic(event event.GenericEvent, q workqueue.RateLimitingInterface) {
	a.process(event.Object.(*apiregistrationv1.APIService), q)
}

func (a *APIServiceHandler) process(apiService *apiregistrationv1.APIService, q workqueue.RateLimitingInterface) {
	if apiService.Name == clusterGatewayAPIServiceName {
		q.Add(reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name: common.AddonName,
			},
		})
	}

}
