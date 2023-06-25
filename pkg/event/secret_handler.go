package event

import (
	"github.com/oam-dev/cluster-gateway/pkg/common"

	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ handler.EventHandler = &SecretHandler{}

type SecretHandler struct {
}

func (s *SecretHandler) Create(_ context.Context, event event.CreateEvent, q workqueue.RateLimitingInterface) {
	s.process(event.Object.(*corev1.Secret), q)
}

func (s *SecretHandler) Update(_ context.Context, event event.UpdateEvent, q workqueue.RateLimitingInterface) {
	s.process(event.ObjectNew.(*corev1.Secret), q)
}

func (s *SecretHandler) Delete(_ context.Context, event event.DeleteEvent, q workqueue.RateLimitingInterface) {
	s.process(event.Object.(*corev1.Secret), q)
}

func (s *SecretHandler) Generic(_ context.Context, event event.GenericEvent, q workqueue.RateLimitingInterface) {
	s.process(event.Object.(*corev1.Secret), q)
}

func (s *SecretHandler) process(secret *corev1.Secret, q workqueue.RateLimitingInterface) {
	for _, ref := range secret.OwnerReferences {
		if ref.Kind == "ManagedServiceAccount" && ref.Name == common.AddonName {
			q.Add(reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: common.AddonName,
				},
			})
		}
	}
}
