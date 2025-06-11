package event

import (
	"github.com/oam-dev/cluster-gateway/pkg/common"

	"context"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ handler.EventHandler = &APIServiceHandler{}

type APIServiceHandler struct {
	WatchingName string
}

func (a *APIServiceHandler) Create(_ context.Context, e event.TypedCreateEvent[client.Object], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	a.process(e.Object.(*apiregistrationv1.APIService), q)
}

func (a *APIServiceHandler) Update(_ context.Context, e event.TypedUpdateEvent[client.Object], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	a.process(e.ObjectNew.(*apiregistrationv1.APIService), q)
}

func (a *APIServiceHandler) Delete(_ context.Context, e event.TypedDeleteEvent[client.Object], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	a.process(e.Object.(*apiregistrationv1.APIService), q)
}

func (a *APIServiceHandler) Generic(_ context.Context, e event.TypedGenericEvent[client.Object], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	a.process(e.Object.(*apiregistrationv1.APIService), q)
}

func (a *APIServiceHandler) process(apiService *apiregistrationv1.APIService, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	if apiService.Name == a.WatchingName {
		q.Add(reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name: common.AddonName,
			},
		})
	}
}
