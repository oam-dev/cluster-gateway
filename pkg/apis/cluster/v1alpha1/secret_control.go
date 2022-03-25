package v1alpha1

import (
	"context"

	"github.com/oam-dev/cluster-gateway/pkg/config"
	"github.com/oam-dev/cluster-gateway/pkg/featuregates"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/kubernetes"
	corev1lister "k8s.io/client-go/listers/core/v1"
)

func getSecretControl() secretControl {
	if utilfeature.DefaultMutableFeatureGate.Enabled(featuregates.SecretCache) {
		return &cachedSecretControl{
			secretNamespace: config.SecretNamespace,
			secretLister:    secretLister,
		}
	}
	return &directApiSecretControl{
		secretNamespace: config.SecretNamespace,
		kubeClient:      kubeClient,
	}
}

type secretControl interface {
	Get(ctx context.Context, name string) (*corev1.Secret, error)
	List(ctx context.Context) ([]*corev1.Secret, error)
}

var _ secretControl = &directApiSecretControl{}

type directApiSecretControl struct {
	secretNamespace string
	kubeClient      kubernetes.Interface
}

func (d *directApiSecretControl) Get(ctx context.Context, name string) (*corev1.Secret, error) {
	return d.kubeClient.CoreV1().Secrets(d.secretNamespace).Get(ctx, name, metav1.GetOptions{})
}

func (d *directApiSecretControl) List(ctx context.Context) ([]*corev1.Secret, error) {
	requirement, err := labels.NewRequirement(
		LabelKeyClusterCredentialType,
		selection.Exists,
		nil)
	if err != nil {
		return nil, err
	}
	secretList, err := d.kubeClient.CoreV1().Secrets(d.secretNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: labels.NewSelector().Add(*requirement).String(),
	})
	if err != nil {
		return nil, err
	}
	secrets := make([]*corev1.Secret, len(secretList.Items))
	for i := range secretList.Items {
		secrets[i] = &secretList.Items[i]
	}
	return secrets, nil
}

var _ secretControl = &cachedSecretControl{}

type cachedSecretControl struct {
	secretNamespace string
	secretLister    corev1lister.SecretLister
}

func (c *cachedSecretControl) Get(ctx context.Context, name string) (*corev1.Secret, error) {
	return c.secretLister.Secrets(c.secretNamespace).Get(name)
}

func (c *cachedSecretControl) List(ctx context.Context) ([]*corev1.Secret, error) {
	requirement, err := labels.NewRequirement(
		LabelKeyClusterCredentialType,
		selection.Exists,
		nil)
	if err != nil {
		return nil, err
	}
	selector := labels.NewSelector().Add(*requirement)
	return c.secretLister.Secrets(c.secretNamespace).List(selector)
}
