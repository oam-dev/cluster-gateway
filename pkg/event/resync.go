package event

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/oam-dev/cluster-gateway/pkg/common"
)

func AddOnHealthResyncHandler(c client.Client, interval time.Duration) source.TypedSource[reconcile.Request] {
	src := StartBackgroundExternalTimerResync(func() ([]event.TypedGenericEvent[client.Object], error) {
		addonList := &addonv1alpha1.ManagedClusterAddOnList{}
		if err := c.List(context.TODO(), addonList); err != nil {
			return nil, err
		}
		evs := make([]event.TypedGenericEvent[client.Object], 0)
		for _, addon := range addonList.Items {
			if addon.Name != common.AddonName {
				continue
			}
			addon := addon
			evs = append(evs, event.TypedGenericEvent[client.Object]{
				Object: &addon,
			})
		}
		return evs, nil
	}, interval)
	return src
}

type GeneratorFunc func() ([]event.TypedGenericEvent[client.Object], error)

func StartBackgroundExternalTimerResync(g GeneratorFunc, interval time.Duration) source.TypedSource[reconcile.Request] {
	events := make(chan event.TypedGenericEvent[client.Object])
	ch := source.Channel[client.Object](events, AddonHealthHandler{})
	ticker := time.NewTicker(interval)
	go func() {
		for {
			_, ok := <-ticker.C
			if !ok {
				return
			}
			evs, err := g()
			if err != nil {
				klog.Errorf("Encountered an error when getting periodic events: %v", err)
				continue
			}
			for _, ev := range evs {
				events <- ev
			}
		}
	}()
	return ch
}

var _ handler.EventHandler = AddonHealthHandler{}

type AddonHealthHandler struct {
}

func (a AddonHealthHandler) Generic(_ context.Context, genericEvent event.TypedGenericEvent[client.Object], limitingInterface workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	limitingInterface.Add(reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: genericEvent.Object.GetNamespace(),
			Name:      genericEvent.Object.GetName(),
		},
	})
}

func (a AddonHealthHandler) Create(_ context.Context, createEvent event.TypedCreateEvent[client.Object], limitingInterface workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	panic("implement me") // unreachable
}

func (a AddonHealthHandler) Update(_ context.Context, updateEvent event.TypedUpdateEvent[client.Object], limitingInterface workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	panic("implement me") // unreachable
}

func (a AddonHealthHandler) Delete(_ context.Context, deleteEvent event.TypedDeleteEvent[client.Object], limitingInterface workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	panic("implement me") // unreachable
}
