package kubernetes

import (
	"context"
	multicluster "github.com/oam-dev/cluster-gateway/pkg/apis/cluster/transport"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/oam-dev/cluster-gateway/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	configmapTestBasename = "configmap-benchmark"
)

var direct bool

var _ = Describe("Basic RoundTrip Test",
	func() {
		f := framework.NewE2EFramework(configmapTestBasename)

		var multiClusterClient kubernetes.Interface
		var err error

		cfg := f.HubRESTConfig()
		cfg.RateLimiter = nil
		if !direct {
			cfg.WrapTransport = multicluster.NewClusterGatewayRoundTripper
		}
		multiClusterClient, err = kubernetes.NewForConfig(cfg)
		Expect(err).NotTo(HaveOccurred())

		var targetConfigMapName = "cluster-gateway-e2e-" + framework.RunID
		var targetConfigMapNamespace = "default"

		Measure("it should do something hard efficiently", func(b Benchmarker) {
			runtime := b.Time("create-update-delete", func() {

				creatingConfigMap := &corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: targetConfigMapNamespace,
						Name:      targetConfigMapName,
					},
					Data: map[string]string{
						"version": "1",
					},
				}
				createdConfigMap, err := multiClusterClient.CoreV1().
					ConfigMaps(targetConfigMapNamespace).
					Create(
						multicluster.WithMultiClusterContext(context.TODO(), f.TestClusterName()),
						creatingConfigMap,
						metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())

				createdConfigMap.Data["version"] = "2"
				_, err = multiClusterClient.CoreV1().
					ConfigMaps(targetConfigMapNamespace).
					Update(
						multicluster.WithMultiClusterContext(context.TODO(), f.TestClusterName()),
						createdConfigMap,
						metav1.UpdateOptions{})
				Expect(err).NotTo(HaveOccurred())

				err = multiClusterClient.CoreV1().
					ConfigMaps(targetConfigMapNamespace).
					Delete(
						multicluster.WithMultiClusterContext(context.TODO(), f.TestClusterName()),
						targetConfigMapName,
						metav1.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			})

			Ω(runtime.Seconds()).
				Should(
					BeNumerically("<", 15),
					"shouldn't take too long.")
		}, 100)


		Measure("get namespace kube-system from managed cluster", func(b Benchmarker) {
			runtime := b.Time("runtime", func() {
				_, err = multiClusterClient.CoreV1().Namespaces().Get(
					multicluster.WithMultiClusterContext(context.TODO(), f.TestClusterName()), "kube-system", metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
			})

			Ω(runtime.Seconds()).Should(BeNumerically("<", 15))

		}, 1000)

		Measure("list namespace from managed cluster", func(b Benchmarker) {
			runtime := b.Time("runtime", func() {
				_, err = multiClusterClient.CoreV1().Namespaces().List(
					multicluster.WithMultiClusterContext(context.TODO(), f.TestClusterName()), metav1.ListOptions{Limit: 100})
				Expect(err).NotTo(HaveOccurred())
			})

			Ω(runtime.Seconds()).Should(BeNumerically("<", 15))

		}, 1000)
	})
