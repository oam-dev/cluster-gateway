package event

import (
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

func (s *SecretHandler) Create(event event.CreateEvent, q workqueue.RateLimitingInterface) {
	s.process(event.Object.(*corev1.Secret), q)
}

func (s *SecretHandler) Update(event event.UpdateEvent, q workqueue.RateLimitingInterface) {
	s.process(event.ObjectNew.(*corev1.Secret), q)
}

func (s *SecretHandler) Delete(event event.DeleteEvent, q workqueue.RateLimitingInterface) {
	s.process(event.Object.(*corev1.Secret), q)
}

func (s *SecretHandler) Generic(event event.GenericEvent, q workqueue.RateLimitingInterface) {
	s.process(event.Object.(*corev1.Secret), q)
}

func (s *SecretHandler) process(secret *corev1.Secret, q workqueue.RateLimitingInterface) {
	for _, ref := range secret.OwnerReferences {
		if ref.Kind == "ClusterManagementAddOn" {
			q.Add(reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: ref.Name,
				},
			})
		}
	}
}
