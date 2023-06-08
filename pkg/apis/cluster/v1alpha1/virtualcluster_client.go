/*
Copyright 2023 The KubeVela Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/utils/strings/slices"
	ocmclusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/oam-dev/cluster-register/pkg/spoke"

	"github.com/oam-dev/cluster-gateway/pkg/common"
	"github.com/oam-dev/cluster-gateway/pkg/config"
)

// VirtualClusterClient client for reading cluster information
// +kubebuilder:object:generate=false
type VirtualClusterClient interface {
	Get(ctx context.Context, name string) (*VirtualCluster, error)
	List(ctx context.Context, options ...client.ListOption) (*VirtualClusterList, error)

	UpdateStatus(ctx context.Context, name string, status VirtualClusterStatus) (*VirtualCluster, error)

	Alias(ctx context.Context, name string, alias string) error
	AddLabels(ctx context.Context, name string, labels map[string]string) (*VirtualCluster, error)
	RemoveLabels(ctx context.Context, name string, labels []string) (*VirtualCluster, error)
	Detach(ctx context.Context, name string, options ...DetachClusterOption) (*VirtualCluster, error)
	DetachCluster(ctx context.Context, vc *VirtualCluster, options ...DetachClusterOption) error
	Rename(ctx context.Context, name string, newName string) (*VirtualCluster, error)
	RenameCluster(ctx context.Context, vc *VirtualCluster, newName string) (*VirtualCluster, error)
}

type virtualClusterClient struct {
	client.Client
	namespace        string
	withControlPlane bool
}

// NewVirtualClusterClient create a client for accessing cluster
func NewVirtualClusterClient(cli client.Client, namespace string, withControlPlane bool) VirtualClusterClient {
	return &virtualClusterClient{Client: cli, namespace: namespace, withControlPlane: withControlPlane}
}

func (c *virtualClusterClient) Get(ctx context.Context, name string) (*VirtualCluster, error) {
	if name == ClusterLocalName {
		return NewLocalVirtualCluster(), nil
	}
	var cluster *VirtualCluster
	secret := &corev1.Secret{}
	err := c.Client.Get(ctx, types.NamespacedName{Name: name, Namespace: c.namespace}, secret)
	var secretErr error
	if err == nil {
		if cluster, secretErr = NewVirtualClusterFromSecret(secret); secretErr == nil {
			return cluster, nil
		}
	}
	if err != nil && !apierrors.IsNotFound(err) {
		secretErr = err
	}

	managedCluster := &ocmclusterv1.ManagedCluster{}
	err = c.Client.Get(ctx, types.NamespacedName{Name: name}, managedCluster)
	var managedClusterErr error
	if err == nil {
		if cluster, managedClusterErr = NewVirtualClusterFromManagedCluster(managedCluster); managedClusterErr == nil {
			return cluster, nil
		}
	}

	if err != nil && !apierrors.IsNotFound(err) && !meta.IsNoMatchError(err) && !runtime.IsNotRegisteredError(err) {
		managedClusterErr = err
	}

	errs := utilerrors.NewAggregate([]error{secretErr, managedClusterErr})
	if errs == nil {
		return nil, apierrors.NewNotFound(schema.GroupResource{
			Group:    config.MetaApiGroupName,
			Resource: "virtualclusters",
		}, name)
	} else if len(errs.Errors()) == 1 {
		return nil, errs.Errors()[0]
	} else {
		return nil, errs
	}
}

func (c *virtualClusterClient) List(ctx context.Context, options ...client.ListOption) (*VirtualClusterList, error) {
	opts := &client.ListOptions{}
	for _, opt := range options {
		opt.ApplyToList(opts)
	}
	clusters := &VirtualClusterList{}
	if c.withControlPlane {
		clusters.Items = append(clusters.Items, *NewLocalVirtualCluster())
	}

	secrets := &corev1.SecretList{}
	err := c.Client.List(ctx, secrets, virtualClusterSelector{selector: opts.LabelSelector, requireCredentialType: true, namespace: c.namespace})
	if err != nil {
		return nil, err
	}
	for _, secret := range secrets.Items {
		if cluster, err := NewVirtualClusterFromSecret(secret.DeepCopy()); err == nil {
			if !clusters.HasCluster(cluster.Name) {
				clusters.Items = append(clusters.Items, *cluster)
			}
		}
	}

	managedClusters := &ocmclusterv1.ManagedClusterList{}
	err = c.Client.List(ctx, managedClusters, virtualClusterSelector{selector: opts.LabelSelector, requireCredentialType: false})
	if err != nil && !meta.IsNoMatchError(err) && !runtime.IsNotRegisteredError(err) {
		return nil, err
	}
	for _, managedCluster := range managedClusters.Items {
		if !clusters.HasCluster(managedCluster.Name) {
			if cluster, err := NewVirtualClusterFromManagedCluster(managedCluster.DeepCopy()); err == nil {
				clusters.Items = append(clusters.Items, *cluster)
			}
		}
	}

	// filter clusters
	var items []VirtualCluster
	for _, cluster := range clusters.Items {
		if opts.LabelSelector == nil || opts.LabelSelector.Matches(labels.Set(cluster.GetLabels())) {
			items = append(items, cluster)
		}
	}
	clusters.Items = items

	// sort clusters
	sort.Slice(clusters.Items, func(i, j int) bool {
		if clusters.Items[i].Name == ClusterLocalName {
			return true
		} else if clusters.Items[j].Name == ClusterLocalName {
			return false
		} else {
			return clusters.Items[i].CreationTimestamp.After(clusters.Items[j].CreationTimestamp.Time)
		}
	})
	return clusters, nil
}

// detachClusterConfig config for detaching cluster
type detachClusterConfig struct {
	managedClusterKubeConfigPath string
}

func newDetachClusterConfig(options ...DetachClusterOption) *detachClusterConfig {
	args := &detachClusterConfig{}
	for _, op := range options {
		op.ApplyTo(args)
	}
	return args
}

// DetachClusterOption option for detach cluster
// +kubebuilder:object:generate=false
type DetachClusterOption interface {
	ApplyTo(cfg *detachClusterConfig)
}

// DetachClusterManagedClusterKubeConfigPathOption configure the managed cluster kubeconfig path while detach ocm cluster
type DetachClusterManagedClusterKubeConfigPathOption string

// ApplyTo apply to args
func (op DetachClusterManagedClusterKubeConfigPathOption) ApplyTo(cfg *detachClusterConfig) {
	cfg.managedClusterKubeConfigPath = string(op)
}

// UpdateStatus update the status of the virtual cluster
func (c *virtualClusterClient) UpdateStatus(ctx context.Context, name string, status VirtualClusterStatus) (*VirtualCluster, error) {
	vc, err := c.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	if vc.Raw == nil {
		return nil, fmt.Errorf("no underlying object")
	}
	annot := vc.Raw.GetAnnotations()
	if annot == nil {
		annot = map[string]string{}
	}
	c.encodeStatusField(status.Version, AnnotationClusterVersion, annot)
	c.encodeStatusField(status.VirtualClusterHealthiness, AnnotationClusterHealth, annot)
	c.encodeStatusField(status.Resources, AnnotationClusterResources, annot)
	vc.Raw.SetAnnotations(annot)
	vc.Status = status
	if err = c.Client.Update(ctx, vc.Raw); err != nil {
		return nil, err
	}
	return vc, err
}

func (c *virtualClusterClient) encodeStatusField(obj interface{}, key string, annot map[string]string) {
	if obj == nil {
		delete(annot, key)
	}
	if bs, err := json.Marshal(obj); err == nil {
		annot[key] = string(bs)
	} else {
		delete(annot, key)
	}
}

func (c *virtualClusterClient) Alias(ctx context.Context, name string, alias string) error {
	vc, err := c.Get(ctx, name)
	if err != nil {
		return err
	}
	switch vc.Spec.CredentialType {
	case CredentialTypeX509Certificate, CredentialTypeServiceAccountToken, CredentialTypeOCMManagedCluster:
		break
	case CredentialTypeInternal:
		return fmt.Errorf("internal cluster cannot be aliased")
	default:
		return fmt.Errorf("unrecognizable credential type %T for cluster %s", vc.Spec.CredentialType, name)
	}

	annot := vc.Raw.GetAnnotations()
	if annot == nil {
		annot = map[string]string{}
	}
	annot[AnnotationClusterAlias] = alias
	vc.Raw.SetAnnotations(annot)
	return c.Client.Update(ctx, vc.Raw)
}

func (c *virtualClusterClient) AddLabels(ctx context.Context, name string, labels map[string]string) (*VirtualCluster, error) {
	vc, err := c.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	switch vc.Spec.CredentialType {
	case CredentialTypeInternal:
		return vc, fmt.Errorf("internal cluster cannot set labels for now")
	}
	ls := vc.Raw.GetLabels()
	for k, v := range labels {
		if !strings.HasPrefix(k, config.MetaApiGroupName) {
			ls[k] = v
		}
	}
	vc.Raw.SetLabels(ls)
	if err = c.Client.Update(ctx, vc.Raw); err != nil {
		return nil, err
	}
	return NewVirtualClusterFromObject(vc.Raw)
}

func (c *virtualClusterClient) RemoveLabels(ctx context.Context, name string, labels []string) (*VirtualCluster, error) {
	vc, err := c.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	switch vc.Spec.CredentialType {
	case CredentialTypeInternal:
		return vc, fmt.Errorf("internal cluster cannot set labels for now")
	}
	ls := vc.Raw.GetLabels()
	for _, k := range labels {
		if !strings.HasPrefix(k, config.MetaApiGroupName) {
			delete(ls, k)
		}
	}
	vc.Raw.SetLabels(ls)
	if err = c.Client.Update(ctx, vc.Raw); err != nil {
		return nil, err
	}
	return NewVirtualClusterFromObject(vc.Raw)
}

func (c *virtualClusterClient) Rename(ctx context.Context, name string, newName string) (*VirtualCluster, error) {
	vc, err := c.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	return c.RenameCluster(ctx, vc, newName)
}

func (c *virtualClusterClient) RenameCluster(ctx context.Context, vc *VirtualCluster, newName string) (*VirtualCluster, error) {
	switch vc.Spec.CredentialType {
	case CredentialTypeX509Certificate, CredentialTypeServiceAccountToken:
		break
	case CredentialTypeOCMManagedCluster:
		return nil, fmt.Errorf("rename ocm managed cluster unsupported")
	case CredentialTypeInternal:
		return nil, fmt.Errorf("internal cluster cannot be renamed")
	default:
		return nil, fmt.Errorf("unrecognizable credential type %T for cluster %s", vc.Spec.CredentialType, vc.Name)
	}
	if _, err := c.Get(ctx, newName); err == nil {
		return nil, fmt.Errorf("cluster %s already exists", newName)
	} else if !apierrors.IsNotFound(err) {
		return nil, err
	}

	// rename secret
	secret, ok := vc.Raw.(*corev1.Secret)
	if !ok {
		return nil, fmt.Errorf("unexpected underlying cluster type %T", vc.Raw)
	}
	newClusterSecret := secret.DeepCopy()
	newClusterSecret.SetName(newName)
	newClusterSecret.SetResourceVersion("")
	if err := c.Client.Create(ctx, newClusterSecret); err != nil {
		return nil, fmt.Errorf("failed to create cluster %s: %w", newName, err)
	}
	if err := c.Client.Delete(ctx, secret); err != nil {
		return nil, fmt.Errorf("failed to detach old cluster %s: %w", vc.Name, err)
	}
	return NewVirtualClusterFromSecret(secret)
}

func (c *virtualClusterClient) Detach(ctx context.Context, name string, options ...DetachClusterOption) (*VirtualCluster, error) {
	vc, err := c.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	return vc, c.DetachCluster(ctx, vc, options...)
}

func (c *virtualClusterClient) DetachCluster(ctx context.Context, vc *VirtualCluster, options ...DetachClusterOption) error {
	cfg := newDetachClusterConfig(options...)
	switch vc.Spec.CredentialType {
	case CredentialTypeX509Certificate, CredentialTypeServiceAccountToken:
		return c.Client.Delete(ctx, vc.Raw)
	case CredentialTypeOCMManagedCluster:
		if cfg.managedClusterKubeConfigPath == "" {
			return fmt.Errorf("kubeconfig-path must be set to detach ocm managed cluster")
		}
		apiConfig, err := clientcmd.LoadFromFile(cfg.managedClusterKubeConfigPath)
		if err != nil {
			return err
		}
		restConfig, err := clientcmd.BuildConfigFromKubeconfigGetter("", func() (*clientcmdapi.Config, error) {
			return apiConfig, nil
		})
		if err != nil {
			return err
		}
		if err = spoke.CleanSpokeClusterEnv(restConfig); err != nil {
			return err
		}
		managedCluster := &ocmclusterv1.ManagedCluster{ObjectMeta: metav1.ObjectMeta{Name: vc.Name}}
		return client.IgnoreNotFound(c.Client.Delete(ctx, managedCluster))
	case CredentialTypeInternal:
		return fmt.Errorf("cannot delete internal cluster `local`")
	default:
		return fmt.Errorf("unrecognizable credential type %s for cluster %s", vc.Spec.CredentialType, vc.Name)
	}
}

// virtualClusterSelector filters the list/delete operation of cluster list
type virtualClusterSelector struct {
	selector              labels.Selector
	requireCredentialType bool
	namespace             string
}

// ApplyToList applies this configuration to the given list options.
func (m virtualClusterSelector) ApplyToList(opts *client.ListOptions) {
	opts.LabelSelector = labels.NewSelector()
	if m.selector != nil {
		requirements, _ := m.selector.Requirements()
		for _, r := range requirements {
			if !slices.Contains([]string{LabelClusterControlPlane}, r.Key()) {
				opts.LabelSelector = opts.LabelSelector.Add(r)
			}
		}
	}
	if m.requireCredentialType {
		r, _ := labels.NewRequirement(common.LabelKeyClusterCredentialType, selection.Exists, nil)
		opts.LabelSelector = opts.LabelSelector.Add(*r)
	}
	opts.Namespace = m.namespace
}
