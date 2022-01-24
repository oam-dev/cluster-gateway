package roundtrip

import (
	"context"

	"github.com/oam-dev/cluster-gateway/e2e/framework"
	clusterv1alpha1 "github.com/oam-dev/cluster-gateway/pkg/apis/cluster/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "open-cluster-management.io/api/cluster/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	roundtripTestBasename = "roundtrip"
)

var _ = Describe("Basic RoundTrip Test", func() {
	f := framework.NewE2EFramework(roundtripTestBasename)

	It("ClusterGateway in the API discovery",
		func() {
			By("Discovering ClusterGateway")
			nativeClient := f.HubNativeClient()
			resources, err := nativeClient.Discovery().
				ServerResourcesForGroupVersion("cluster.core.oam.dev/v1alpha1")
			Expect(err).NotTo(HaveOccurred())
			apiFound := false
			for _, resource := range resources.APIResources {
				if resource.Kind == "ClusterGateway" {
					apiFound = true
				}
			}
			if !apiFound {
				Fail(`Api ClusterGateway not found`)
			}
		})

	It("ManagedCluster present",
		func() {
			By("Getting ManagedCluster")
			if f.IsOCMInstalled() {
				runtimeClient := f.HubRuntimeClient()
				cluster := &clusterv1.ManagedCluster{}
				err := runtimeClient.Get(context.TODO(), types.NamespacedName{
					Name: f.TestClusterName(),
				}, cluster)
				Expect(err).NotTo(HaveOccurred())
			}
		})

	It("ClusterGateway can be read via GET",
		func() {
			By("Getting ClusterGateway")
			runtimeClient := f.HubRuntimeClient()
			clusterGateway := &clusterv1alpha1.ClusterGateway{}
			err := runtimeClient.Get(context.TODO(), types.NamespacedName{
				Name: f.TestClusterName(),
			}, clusterGateway)
			Expect(err).NotTo(HaveOccurred())
		})

	It("ClusterGateway can be read via LIST",
		func() {
			By("Getting ClusterGateway")
			runtimeClient := f.HubRuntimeClient()
			clusterGatewayList := &clusterv1alpha1.ClusterGatewayList{}
			err := runtimeClient.List(context.TODO(), clusterGatewayList)
			Expect(err).NotTo(HaveOccurred())
			clusterFound := false
			for _, clusterGateway := range clusterGatewayList.Items {
				if clusterGateway.Name == f.TestClusterName() {
					clusterFound = true
				}
			}
			if !clusterFound {
				Fail(`ClusterGateway not found`)
			}
		})

	It("ClusterGateway healthiness can be manipulated",
		func() {
			By("get healthiness")
			gw, err := f.HubGatewayClient().
				ClusterV1alpha1().
				ClusterGateways().
				GetHealthiness(context.TODO(), f.TestClusterName(), metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(gw).ShouldNot(BeNil())
			Expect(gw.Status.Healthy).To(BeFalse())
			By("update healthiness")
			gw.Status.Healthy = true
			gw.Status.HealthyReason = clusterv1alpha1.HealthyReasonTypeConnectionTimeout
			updated, err := f.HubGatewayClient().
				ClusterV1alpha1().
				ClusterGateways().
				UpdateHealthiness(context.TODO(), gw, metav1.UpdateOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(updated).NotTo(BeNil())
			Expect(updated.Status.Healthy).To(BeTrue())
			Expect(updated.Status.HealthyReason).To(Equal(clusterv1alpha1.HealthyReasonTypeConnectionTimeout))
		})

})
