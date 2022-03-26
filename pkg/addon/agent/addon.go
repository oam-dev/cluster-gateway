package agent

import (
	"context"

	"github.com/pkg/errors"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"open-cluster-management.io/addon-framework/pkg/agent"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	proxyv1alpha1 "github.com/oam-dev/cluster-gateway/pkg/apis/proxy/v1alpha1"
	"github.com/oam-dev/cluster-gateway/pkg/common"
)

var _ agent.AgentAddon = &clusterGatewayAddonManager{}

func NewClusterGatewayAddonManager(cfg *rest.Config, c client.Client) agent.AgentAddon {
	return &clusterGatewayAddonManager{
		clientConfig: cfg,
		client:       c,
	}
}

type clusterGatewayAddonManager struct {
	clientConfig *rest.Config
	client       client.Client
}

func (c *clusterGatewayAddonManager) Manifests(cluster *clusterv1.ManagedCluster, addon *addonv1alpha1.ManagedClusterAddOn) ([]runtime.Object, error) {
	if len(addon.Status.AddOnConfiguration.CRName) == 0 {
		return nil, nil
	}
	cfg := &proxyv1alpha1.ClusterGatewayConfiguration{}
	if err := c.client.Get(
		context.TODO(), types.NamespacedName{
			Name: addon.Status.AddOnConfiguration.CRName,
		},
		cfg); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, errors.Wrapf(err, "failed getting gateway configuration")
	}

	if cfg.Spec.SecretManagement.Type == proxyv1alpha1.SecretManagementTypeManual {
		return nil, nil
	}
	switch cfg.Spec.SecretManagement.Type {
	case proxyv1alpha1.SecretManagementTypeManagedServiceAccount:
		managedServiceAccountAddon := &addonv1alpha1.ManagedClusterAddOn{}
		if err := c.client.Get(
			context.TODO(),
			types.NamespacedName{},
			managedServiceAccountAddon); err != nil {
			if apierrors.IsNotFound(err) {
				return nil, nil
			}
			return nil, err
		}
		return buildClusterGatewayOutboundPermission(
			managedServiceAccountAddon.Spec.InstallNamespace,
			cfg.Spec.SecretManagement.ManagedServiceAccount.Name), nil
	case proxyv1alpha1.SecretManagementTypeManual:
		fallthrough
	default:
		return nil, nil
	}
}

func (c *clusterGatewayAddonManager) GetAgentAddonOptions() agent.AgentAddonOptions {
	return agent.AgentAddonOptions{
		AddonName:       common.AddonName,
		InstallStrategy: agent.InstallAllStrategy(common.InstallNamespace),
		HealthProber: &agent.HealthProber{
			Type: agent.HealthProberTypeNone, // TODO: switch to ManifestWork-based prober
		},
	}
}

func buildClusterGatewayOutboundPermission(serviceAccountNamespace, serviceAccountName string) []runtime.Object {
	const clusterRoleName = "open-cluster-management:cluster-gateway:default"
	clusterGatewayClusterRole := &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleName,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"*"},
				Verbs:     []string{"*"},
				Resources: []string{"*"},
			},
		},
	}
	clusterGatewayClusterRoleBinding := &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleName,
		},
		RoleRef: rbacv1.RoleRef{
			Kind: "ClusterRole",
			Name: clusterRoleName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Namespace: serviceAccountNamespace,
				Name:      serviceAccountName,
			},
		},
	}
	return []runtime.Object{
		clusterGatewayClusterRole,
		clusterGatewayClusterRoleBinding,
	}
}
