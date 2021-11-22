package agent

import (
	"context"
	"fmt"
	"time"

	proxyv1alpha1 "github.com/oam-dev/cluster-gateway/pkg/apis/proxy/v1alpha1"
	"github.com/oam-dev/cluster-gateway/pkg/common"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"open-cluster-management.io/addon-framework/pkg/agent"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	ocmauthv1alpha1 "open-cluster-management.io/managed-serviceaccount/api/v1alpha1"
	"open-cluster-management.io/managed-serviceaccount/pkg/addon/agent/health"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
		return nil, fmt.Errorf("no gateway configuration bond to ManagedClusterAddOn")
	}
	gatewayConfig := &proxyv1alpha1.ClusterGatewayConfiguration{}
	if err := c.client.Get(context.TODO(), types.NamespacedName{
		Name: addon.Status.AddOnConfiguration.CRName,
	}, gatewayConfig); err != nil {
		return nil, fmt.Errorf("failed getting gateway configuration bond to ManagedClusterAddOn")
	}
	endpointType := "Const"
	if gatewayConfig.Spec.Egress.Type == proxyv1alpha1.EgressTypeClusterProxy {
		endpointType = "ClusterProxy"
	}
	msa := &ocmauthv1alpha1.ManagedServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "authentication.open-cluster-management.io/v1alpha1",
			Kind:       "ManagedServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Name,
			Name:      common.AddonName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: addonv1alpha1.GroupVersion.String(),
					Kind:       "ManagedClusterAddOn",
					UID:        addon.UID,
					Name:       addon.Name,
				},
			},
		},
		Spec: ocmauthv1alpha1.ManagedServiceAccountSpec{
			Projected: ocmauthv1alpha1.ManagedServiceAccountProjected{
				Type: ocmauthv1alpha1.ProjectionTypeSecret,
				Secret: &ocmauthv1alpha1.ProjectedSecret{
					Labels: map[string]string{
						"gateway.proxy.open-cluster-management.io/cluster-credential-type": "ServiceAccountToken",
						"gateway.proxy.open-cluster-management.io/cluster-endpoint-type":   endpointType,
					},
					Namespace: gatewayConfig.Spec.SecretNamespace,
					Name:      cluster.Name,
				},
			},
			Rotation: ocmauthv1alpha1.ManagedServiceAccountRotation{
				Enabled: true,
				Validity: metav1.Duration{
					Duration: time.Hour * 24 * 180,
				},
			},
		},
	}
	if err := c.client.Create(context.TODO(), msa); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return nil, errors.Wrapf(err, "failed creating managed serviceaccount for cluster %v", cluster.Name)
		}
	}

	leaseUpdater, err := health.NewAddonHealthUpdater(c.clientConfig, cluster.Name)
	if err != nil {
		return nil, errors.Wrapf(err, "failed creating lease updater")
	}
	go leaseUpdater.Start(context.Background())

	return nil, nil
}

func (c *clusterGatewayAddonManager) GetAgentAddonOptions() agent.AgentAddonOptions {
	return agent.AgentAddonOptions{
		AddonName:       common.AddonName,
		InstallStrategy: agent.InstallAllStrategy(common.InstallNamespace),
	}
}
