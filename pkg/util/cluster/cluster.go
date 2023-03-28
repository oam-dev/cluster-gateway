package cluster

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ocmclient "open-cluster-management.io/api/client/cluster/clientset/versioned"
	clusterv1Lister "open-cluster-management.io/api/client/cluster/listers/cluster/v1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
)

type OCMClusterControl interface {
	Get(ctx context.Context, name string) (*clusterv1.ManagedCluster, error)
	List(ctx context.Context) ([]*clusterv1.ManagedCluster, error)
}

var _ OCMClusterControl = &directOCMClusterControl{}

type directOCMClusterControl struct {
	ocmClient ocmclient.Interface
}

func NewDirectOCMClusterControl(ocmClient ocmclient.Interface) OCMClusterControl {
	return &directOCMClusterControl{
		ocmClient: ocmClient,
	}
}

func (c *directOCMClusterControl) Get(ctx context.Context, name string) (*clusterv1.ManagedCluster, error) {
	return c.ocmClient.ClusterV1().ManagedClusters().Get(ctx, name, metav1.GetOptions{})
}

func (c *directOCMClusterControl) List(ctx context.Context) ([]*clusterv1.ManagedCluster, error) {
	clusterList, err := c.ocmClient.ClusterV1().ManagedClusters().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	clusters := make([]*clusterv1.ManagedCluster, len(clusterList.Items))
	for _, item := range clusters {
		clusters = append(clusters, item)
	}

	return clusters, nil
}

var _ OCMClusterControl = &cacheOCMClusterControl{}

type cacheOCMClusterControl struct {
	clusterLister clusterv1Lister.ManagedClusterLister
}

func NewCacheOCMClusterControl(clusterLister clusterv1Lister.ManagedClusterLister) OCMClusterControl {
	return &cacheOCMClusterControl{
		clusterLister: clusterLister,
	}
}

func (c *cacheOCMClusterControl) Get(ctx context.Context, name string) (*clusterv1.ManagedCluster, error) {
	return c.clusterLister.Get(name)
}

func (c *cacheOCMClusterControl) List(ctx context.Context) ([]*clusterv1.ManagedCluster, error) {
	return c.clusterLister.List(labels.Everything())
}

// IsOCMManagedClusterInstalled check if managed cluster is installed in the cluster
func IsOCMManagedClusterInstalled(ocmClient ocmclient.Interface) (bool, error) {
	_, resources, err := ocmClient.Discovery().ServerGroupsAndResources()
	if err != nil {
		return false, fmt.Errorf("unable to get api-resources: %w", err)
	}
	for _, resource := range resources {
		if gv, _ := schema.ParseGroupVersion(resource.GroupVersion); gv.Group == clusterv1.GroupName {
			for _, rsc := range resource.APIResources {
				if rsc.Kind == "ManagedCluster" {
					return true, nil
				}
			}
		}
	}
	return false, nil
}
