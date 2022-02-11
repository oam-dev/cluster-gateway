package event

import (
	"context"
	"time"

	"github.com/oam-dev/cluster-gateway/pkg/common"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

func AddOnHealthResyncHandler(c client.Client, interval time.Duration) (*source.Channel, handler.EventHandler) {
	ch := StartBackgroundExternalTimerResync(func() ([]event.GenericEvent, error) {
		addonList := &addonv1alpha1.ManagedClusterAddOnList{}
		if err := c.List(context.TODO(), addonList); err != nil {
			return nil, err
		}
		evs := make([]event.GenericEvent, 0)
		for _, addon := range addonList.Items {
			if addon.Name != common.AddonName {
				continue
			}
			addon := addon
			evs = append(evs, event.GenericEvent{
				Object: &addon,
			})
		}
		return evs, nil
	}, interval)
	return ch, AddonHealthHandler{}
}

type GeneratorFunc func() ([]event.GenericEvent, error)

func StartBackgroundExternalTimerResync(g GeneratorFunc, interval time.Duration) *source.Channel {
	events := make(chan event.GenericEvent) // unbuffered
	ch := &source.Channel{Source: events}
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

var _ handler.EventHandler = &AddonHealthHandler{}

type AddonHealthHandler struct {
}

func (a AddonHealthHandler) Generic(genericEvent event.GenericEvent, limitingInterface workqueue.RateLimitingInterface) {
	limitingInterface.Add(reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: genericEvent.Object.GetNamespace(),
			Name:      genericEvent.Object.GetName(),
		},
	})
}

func (a AddonHealthHandler) Create(createEvent event.CreateEvent, limitingInterface workqueue.RateLimitingInterface) {
	panic("implement me") // unreachable
}

func (a AddonHealthHandler) Update(updateEvent event.UpdateEvent, limitingInterface workqueue.RateLimitingInterface) {
	panic("implement me") // unreachable
}

func (a AddonHealthHandler) Delete(deleteEvent event.DeleteEvent, limitingInterface workqueue.RateLimitingInterface) {
	panic("implement me") // unreachable
}
