package roundtrip

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"

	"github.com/oam-dev/cluster-gateway/e2e/framework"
	"github.com/oam-dev/cluster-gateway/pkg/common"
)

const (
	ocmTestBasename = "ocm-addon"
)

var _ = Describe("Addon Manager Test", func() {
	f := framework.NewE2EFramework(ocmTestBasename)
	It("ClusterGateway addon installation should work",
		func() {
			c := f.HubRuntimeClient()
			By("Polling addon and gateway healthiness")
			Eventually(
				func() (bool, error) {
					addon := &addonapiv1alpha1.ManagedClusterAddOn{}
					if err := c.Get(context.TODO(), types.NamespacedName{
						Namespace: f.TestClusterName(),
						Name:      common.AddonName,
					}, addon); err != nil {
						if apierrors.IsNotFound(err) {
							return false, nil
						}
						return false, err
					}
					if addon.Status.HealthCheck.Mode != addonapiv1alpha1.HealthCheckModeCustomized {
						return false, nil
					}
					addonHealthy := meta.IsStatusConditionTrue(
						addon.Status.Conditions,
						addonapiv1alpha1.ManagedClusterAddOnConditionAvailable)
					gw, err := f.HubGatewayClient().
						ClusterV1alpha1().
						ClusterGateways().
						GetHealthiness(context.TODO(), f.TestClusterName(), metav1.GetOptions{})
					if err != nil {
						return false, err
					}
					gwHealthy := gw.Status.Healthy
					return addonHealthy && gwHealthy, nil
				}).
				WithTimeout(time.Minute).
				Should(BeTrue())
		})
	It("Manual probe healthiness should work",
		func() {
			resp, err := f.HubNativeClient().Discovery().
				RESTClient().
				Get().
				AbsPath(
					"apis/cluster.core.oam.dev/v1alpha1/clustergateways",
					f.TestClusterName(),
					"proxy",
					"healthz",
				).DoRaw(context.TODO())
			Expect(err).NotTo(HaveOccurred())
			Expect(string(resp)).To(Equal("ok"))
		})
})
