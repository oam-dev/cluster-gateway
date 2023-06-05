package event

import (
	"github.com/oam-dev/cluster-gateway/pkg/common"

	"context"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ handler.EventHandler = &APIServiceHandler{}

type APIServiceHandler struct {
	WatchingName string
}

func (a *APIServiceHandler) Create(_ context.Context, event event.CreateEvent, q workqueue.RateLimitingInterface) {
	a.process(event.Object.(*apiregistrationv1.APIService), q)
}

func (a *APIServiceHandler) Update(_ context.Context, event event.UpdateEvent, q workqueue.RateLimitingInterface) {
	a.process(event.ObjectNew.(*apiregistrationv1.APIService), q)
}

func (a *APIServiceHandler) Delete(_ context.Context, event event.DeleteEvent, q workqueue.RateLimitingInterface) {
	a.process(event.Object.(*apiregistrationv1.APIService), q)
}

func (a *APIServiceHandler) Generic(_ context.Context, event event.GenericEvent, q workqueue.RateLimitingInterface) {
	a.process(event.Object.(*apiregistrationv1.APIService), q)
}

func (a *APIServiceHandler) process(apiService *apiregistrationv1.APIService, q workqueue.RateLimitingInterface) {
	if apiService.Name == a.WatchingName {
		q.Add(reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name: common.AddonName,
			},
		})
	}
}
