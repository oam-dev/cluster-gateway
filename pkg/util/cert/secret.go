package cert

import (
	"bytes"
	"context"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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
