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

package v1alpha1_test

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/pointer"
	ocmclusterv1 "open-cluster-management.io/api/cluster/v1"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/oam-dev/cluster-gateway/pkg/apis/cluster/v1alpha1"
	clustergatewaycommon "github.com/oam-dev/cluster-gateway/pkg/common"
	"github.com/oam-dev/cluster-gateway/pkg/config"
	"github.com/oam-dev/cluster-gateway/pkg/util/scheme"
	"github.com/oam-dev/cluster-gateway/pkg/util/singleton"
)

func TestVirtualCluster(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "VirtualCluster API Test")
}

var testEnv *envtest.Environment
var cli ctrlClient.Client

// Add comment to trigger the failing test


var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.UseDevMode(true), zap.WriteTo(GinkgoWriter)))
	By("Bootstrapping Test Environment")

	testEnv = &envtest.Environment{
		ControlPlaneStartTimeout: time.Minute,
		ControlPlaneStopTimeout:  time.Minute,
		Scheme:                   scheme.Scheme,
		UseExistingCluster:       pointer.Bool(false),
		CRDDirectoryPaths:        []string{"../../../../hack/crd/registration"},
	}
	cfg, err := testEnv.Start()
	Ω(err).To(Succeed())

	cli, err = ctrlClient.New(cfg, ctrlClient.Options{Scheme: scheme.Scheme})
	Ω(err).To(Succeed())
	singleton.SetCtrlClient(cli)
})

var _ = AfterSuite(func() {
	By("Tearing Down the Test Environment")
	Ω(testEnv.Stop()).To(Succeed())
})

var _ = Describe("Test Cluster API", func() {

	It("Test Cluster API", func() {
		c := &v1alpha1.VirtualCluster{}
		c.SetName("example")
		Ω(c.GetFullName()).To(Equal("example"))
		c.Spec.Alias = "alias"
		Ω(c.GetFullName()).To(Equal("example (alias)"))

		By("Test meta info")
		Ω(c.New()).To(Equal(&v1alpha1.VirtualCluster{}))
		Ω(c.NamespaceScoped()).To(BeFalse())
		Ω(c.ShortNames()).To(SatisfyAll(
			ContainElement("vc"),
			ContainElement("vcluster"),
			ContainElement("vclusters"),
			ContainElement("virtual-cluster"),
			ContainElement("virtual-clusters"),
		))
		Ω(c.GetGroupVersionResource().GroupVersion()).To(Equal(v1alpha1.SchemeGroupVersion))
		Ω(c.GetGroupVersionResource().Resource).To(Equal("virtualclusters"))
		Ω(c.IsStorageVersion()).To(BeTrue())
		Ω(c.NewList()).To(Equal(&v1alpha1.VirtualClusterList{}))

		ctx := context.Background()

		By("Create storage namespace")
		Ω(cli.Create(ctx, &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: config.SecretNamespace}})).To(Succeed())

		By("Create cluster secret")
		Ω(cli.Create(ctx, &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: config.SecretNamespace,
				Labels: map[string]string{
					clustergatewaycommon.LabelKeyClusterCredentialType: string(v1alpha1.CredentialTypeX509Certificate),
					clustergatewaycommon.LabelKeyClusterEndpointType:   string(v1alpha1.ClusterEndpointTypeConst),
					"key": "value",
				},
				Annotations: map[string]string{v1alpha1.AnnotationClusterAlias: "test-cluster-alias"},
			},
		})).To(Succeed())
		Ω(cli.Create(ctx, &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cluster-invalid",
				Namespace: config.SecretNamespace,
			},
			Data: map[string][]byte{"endpoint": []byte("127.0.0.1:6443")},
		})).To(Succeed())
		Ω(cli.Create(ctx, &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ocm-cluster",
				Namespace: config.SecretNamespace,
				Labels: map[string]string{
					clustergatewaycommon.LabelKeyClusterCredentialType: string(v1alpha1.CredentialTypeX509Certificate),
				},
			},
		})).To(Succeed())

		By("Test get cluster from cluster secret")
		obj, err := c.Get(ctx, "test-cluster", nil)
		Ω(err).To(Succeed())
		cluster, ok := obj.(*v1alpha1.VirtualCluster)
		Ω(ok).To(BeTrue())
		Ω(cluster.Spec.Alias).To(Equal("test-cluster-alias"))
		Ω(cluster.Spec.CredentialType).To(Equal(v1alpha1.CredentialTypeX509Certificate))
		Ω(cluster.GetLabels()["key"]).To(Equal("value"))

		_, err = c.Get(ctx, "cluster-invalid", nil)
		Ω(err).To(Satisfy(v1alpha1.IsInvalidClusterSecretError))

		By("Create OCM ManagedCluster")
		Ω(cli.Create(ctx, &ocmclusterv1.ManagedCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ocm-bad-cluster",
				Namespace: config.SecretNamespace,
			},
		})).To(Succeed())
		Ω(cli.Create(ctx, &ocmclusterv1.ManagedCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ocm-cluster",
				Namespace: config.SecretNamespace,
				Labels:    map[string]string{"key": "value"},
			},
			Spec: ocmclusterv1.ManagedClusterSpec{
				ManagedClusterClientConfigs: []ocmclusterv1.ClientConfig{{URL: "test-url"}},
			},
		})).To(Succeed())
		Ω(cli.Create(ctx, &ocmclusterv1.ManagedCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: config.SecretNamespace,
				Labels:    map[string]string{"key": "value-dup"},
			},
			Spec: ocmclusterv1.ManagedClusterSpec{
				ManagedClusterClientConfigs: []ocmclusterv1.ClientConfig{{URL: "test-url-dup"}},
			},
		})).To(Succeed())

		By("Test get cluster from OCM managed cluster")
		_, err = c.Get(ctx, "ocm-bad-cluster", nil)
		Ω(err).To(Satisfy(v1alpha1.IsInvalidManagedClusterError))

		obj, err = c.Get(ctx, "ocm-cluster", nil)
		Ω(err).To(Succeed())
		cluster, ok = obj.(*v1alpha1.VirtualCluster)
		Ω(ok).To(BeTrue())
		Expect(cluster.Spec.CredentialType).To(Equal(v1alpha1.CredentialTypeOCMManagedCluster))

		By("Test get local cluster")
		obj, err = c.Get(ctx, "local", nil)
		Ω(err).To(Succeed())
		cluster, ok = obj.(*v1alpha1.VirtualCluster)
		Ω(ok).To(BeTrue())
		Expect(cluster.Spec.CredentialType).To(Equal(v1alpha1.CredentialTypeInternal))

		_, err = c.Get(ctx, "cluster-not-exist", nil)
		Ω(err).To(Satisfy(apierrors.IsNotFound))

		By("Test list clusters")
		objs, err := c.List(ctx, nil)
		Ω(err).To(Succeed())
		clusters, ok := objs.(*v1alpha1.VirtualClusterList)
		Ω(ok).To(BeTrue())
		Expect(len(clusters.Items)).To(Equal(3))
		Expect(clusters.Items[0].Name).To(Equal("local"))
		Expect(clusters.Items[1].Name).To(Equal("ocm-cluster"))
		Expect(clusters.Items[2].Name).To(Equal("test-cluster"))

		By("Test list clusters with labels")
		objs, err = c.List(ctx, &metainternalversion.ListOptions{LabelSelector: labels.SelectorFromSet(map[string]string{"key": "value"})})
		Ω(err).To(Succeed())
		clusters, ok = objs.(*v1alpha1.VirtualClusterList)
		Ω(ok).To(BeTrue())
		Expect(len(clusters.Items)).To(Equal(2))
		Expect(clusters.Items[0].Name).To(Equal("ocm-cluster"))
		Expect(clusters.Items[1].Name).To(Equal("test-cluster"))

		By("Test list clusters that are not control plane")
		objs, err = c.List(ctx, &metainternalversion.ListOptions{LabelSelector: labels.SelectorFromSet(map[string]string{
			v1alpha1.LabelClusterControlPlane: "false",
		})})
		Ω(err).To(Succeed())
		clusters, ok = objs.(*v1alpha1.VirtualClusterList)
		Ω(ok).To(BeTrue())
		Expect(len(clusters.Items)).To(Equal(2))
		Expect(clusters.Items[0].Name).To(Equal("ocm-cluster"))
		Expect(clusters.Items[1].Name).To(Equal("test-cluster"))

		By("Test list clusters that is control plane")
		objs, err = c.List(ctx, &metainternalversion.ListOptions{LabelSelector: labels.SelectorFromSet(map[string]string{
			v1alpha1.LabelClusterControlPlane: "true",
		})})
		Ω(err).To(Succeed())
		clusters, ok = objs.(*v1alpha1.VirtualClusterList)
		Ω(ok).To(BeTrue())
		Expect(len(clusters.Items)).To(Equal(1))
		Expect(clusters.Items[0].Name).To(Equal("local"))

		By("Test print table")
		_, err = c.ConvertToTable(ctx, cluster, nil)
		Ω(err).To(Succeed())
		_, err = c.ConvertToTable(ctx, clusters, nil)
		Ω(err).To(Succeed())
	})

})
