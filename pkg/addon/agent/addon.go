package agent

import (
	"github.com/oam-dev/cluster-gateway/pkg/common"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"open-cluster-management.io/addon-framework/pkg/agent"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
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
	return nil, nil
}

func (c *clusterGatewayAddonManager) GetAgentAddonOptions() agent.AgentAddonOptions {
	return agent.AgentAddonOptions{
		AddonName:       common.AddonName,
		InstallStrategy: agent.InstallAllStrategy(common.InstallNamespace),
	}
}
