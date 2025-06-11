package event

import (
	"github.com/oam-dev/cluster-gateway/pkg/common"

	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ handler.EventHandler = &SecretHandler{}

type SecretHandler struct {
}

func (s *SecretHandler) Create(_ context.Context, e event.TypedCreateEvent[client.Object], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	s.process(e.Object.(*corev1.Secret), q)
}

func (s *SecretHandler) Update(_ context.Context, e event.TypedUpdateEvent[client.Object], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	s.process(e.ObjectNew.(*corev1.Secret), q)
}

func (s *SecretHandler) Delete(_ context.Context, e event.TypedDeleteEvent[client.Object], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	s.process(e.Object.(*corev1.Secret), q)
}

func (s *SecretHandler) Generic(_ context.Context, e event.TypedGenericEvent[client.Object], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	s.process(e.Object.(*corev1.Secret), q)
}

func (s *SecretHandler) process(secret *corev1.Secret, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
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
