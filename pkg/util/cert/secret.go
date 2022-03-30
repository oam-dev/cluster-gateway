package cert

import (
	"bytes"
	"context"

	"github.com/oam-dev/cluster-gateway/pkg/common"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"
	corev1lister "k8s.io/client-go/listers/core/v1"
)

func CopySecret(kubeClient kubernetes.Interface, sourceNamespace string, sourceName string, targetNamespace, targetName string) error {
	sourceSecret, err := kubeClient.CoreV1().
		Secrets(sourceNamespace).
		Get(context.TODO(), sourceName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed getting source secret %v/%v: %v", sourceNamespace, sourceName, err)
	}
	shouldCreate := false
	existingTargetSecret, err := kubeClient.CoreV1().
		Secrets(targetNamespace).
		Get(context.TODO(), targetName, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed getting target secret %v/%v: %v", targetNamespace, targetName, err)
		}
		shouldCreate = true
	}
	if shouldCreate {
		existingTargetSecret = sourceSecret.DeepCopy()
		existingTargetSecret.Namespace = targetNamespace
		existingTargetSecret.Name = targetName
		existingTargetSecret.UID = ""
		existingTargetSecret.ResourceVersion = ""
		if _, err := kubeClient.CoreV1().Secrets(targetNamespace).
			Create(context.TODO(), existingTargetSecret, metav1.CreateOptions{}); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return errors.Wrapf(err, "failed creating CA secret")
			}
		}
		return nil
	}

	if IsSubset(sourceSecret.Data, existingTargetSecret.Data) {
		return nil
	}
	Merge(sourceSecret.Data, existingTargetSecret.Data)
	_, err = kubeClient.CoreV1().
		Secrets(targetNamespace).
		Update(context.TODO(), existingTargetSecret, metav1.UpdateOptions{})
	return err
}

func IsSubset(subset, superset map[string][]byte) bool {
	for k, v := range subset {
		if !bytes.Equal(v, superset[k]) {
			return false
		}
	}
	return true
}

func Merge(l, r map[string][]byte) {
	for k, v := range l {
		r[k] = v
	}
}

type SecretControl interface {
	Get(ctx context.Context, name string) (*corev1.Secret, error)
	List(ctx context.Context) ([]*corev1.Secret, error)
}

var _ SecretControl = &directApiSecretControl{}

type directApiSecretControl struct {
	secretNamespace string
	kubeClient      kubernetes.Interface
}

func (d *directApiSecretControl) Get(ctx context.Context, name string) (*corev1.Secret, error) {
	return d.kubeClient.CoreV1().Secrets(d.secretNamespace).Get(ctx, name, metav1.GetOptions{})
}

func (d *directApiSecretControl) List(ctx context.Context) ([]*corev1.Secret, error) {
	requirement, err := labels.NewRequirement(
		common.LabelKeyClusterCredentialType,
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

var _ SecretControl = &cachedSecretControl{}

type cachedSecretControl struct {
	secretNamespace string
	secretLister    corev1lister.SecretLister
}

func (c *cachedSecretControl) Get(ctx context.Context, name string) (*corev1.Secret, error) {
	return c.secretLister.Secrets(c.secretNamespace).Get(name)
}

func (c *cachedSecretControl) List(ctx context.Context) ([]*corev1.Secret, error) {
	requirement, err := labels.NewRequirement(
		common.LabelKeyClusterCredentialType,
		selection.Exists,
		nil)
	if err != nil {
		return nil, err
	}
	selector := labels.NewSelector().Add(*requirement)
	return c.secretLister.Secrets(c.secretNamespace).List(selector)
}

func NewDirectApiSecretControl(secretNamespace string, kubeClient kubernetes.Interface) SecretControl {
	return &directApiSecretControl{
		secretNamespace: secretNamespace,
		kubeClient:      kubeClient,
	}
}

func NewCachedSecretControl(secretNamespace string, secretLister corev1lister.SecretLister) SecretControl {
	return &cachedSecretControl{
		secretNamespace: secretNamespace,
		secretLister:    secretLister,
	}
}
