package kubernetes

import (
	"context"

	"github.com/oam-dev/cluster-gateway/e2e/framework"
	"github.com/oam-dev/cluster-gateway/pkg/apis/cluster/transport"
	"k8s.io/client-go/kubernetes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	kubernetesTestBasename = "kubernetes"
)

var _ = Describe("Basic RoundTrip Test",
	func() {
		f := framework.NewE2EFramework(kubernetesTestBasename)

		It("Probing cluster health (raw)",
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

		It("Probing cluster health (context)",
			func() {
				cfg := f.HubRESTConfig()
				cfg.WrapTransport = multicluster.NewClusterGatewayRoundTripper
				multiClusterClient, err := kubernetes.NewForConfig(cfg)
				Expect(err).NotTo(HaveOccurred())
				resp, err := multiClusterClient.RESTClient().
					Get().
					AbsPath("healthz").
					DoRaw(multicluster.WithMultiClusterContext(context.TODO(), f.TestClusterName()))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(resp)).To(Equal("ok"))
			})

	})
