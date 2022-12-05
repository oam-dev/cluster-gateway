package singleton

import (
	"time"

	"github.com/oam-dev/cluster-gateway/pkg/config"
	"github.com/oam-dev/cluster-gateway/pkg/featuregates"
	"github.com/oam-dev/cluster-gateway/pkg/util/cert"
	clusterutil "github.com/oam-dev/cluster-gateway/pkg/util/cluster"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/server"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	corev1informer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corev1lister "k8s.io/client-go/listers/core/v1"
	clientgorest "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	ocmclient "open-cluster-management.io/api/client/cluster/clientset/versioned"
	clusterinformers "open-cluster-management.io/api/client/cluster/informers/externalversions"
	clusterv1Lister "open-cluster-management.io/api/client/cluster/listers/cluster/v1"
	"sigs.k8s.io/apiserver-runtime/pkg/util/loopback"
)

var kubeClient kubernetes.Interface
var ocmClient ocmclient.Interface

var secretInformer cache.SharedIndexInformer
var secretLister corev1lister.SecretLister

var secretControl cert.SecretControl

var clusterInformer cache.SharedIndexInformer
var clusterLister clusterv1Lister.ManagedClusterLister
var clusterControl clusterutil.OCMClusterControl

func GetSecretControl() cert.SecretControl {
	return secretControl
}

func GetOCMClient() ocmclient.Interface {
	return ocmClient
}

func GetKubeClient() kubernetes.Interface {
	return kubeClient
}

func InitLoopbackClient(ctx server.PostStartHookContext) error {
	var err error
	copiedCfg := clientgorest.CopyConfig(loopback.GetLoopbackMasterClientConfig())
	copiedCfg.RateLimiter = nil
	kubeClient, err = kubernetes.NewForConfig(copiedCfg)
	if err != nil {
		return err
	}
	ocmClient, err = ocmclient.NewForConfig(copiedCfg)
	if err != nil {
		return err
	}
	if utilfeature.DefaultMutableFeatureGate.Enabled(featuregates.SecretCache) {
		if err := setInformer(kubeClient, ctx.StopCh); err != nil {
			return err
		}
		secretControl = cert.NewCachedSecretControl(config.SecretNamespace, secretLister)
	}
	if secretControl == nil {
		secretControl = cert.NewDirectApiSecretControl(config.SecretNamespace, kubeClient)
	}

	if utilfeature.DefaultMutableFeatureGate.Enabled(featuregates.OCMClusterCache) {
		if err := setOCMClusterInformer(ocmClient, ctx.StopCh); err != nil {
			return err
		}
		clusterControl = clusterutil.NewCacheOCMClusterControl(clusterLister)
	}
	if clusterControl == nil {
		clusterControl = clusterutil.NewDirectOCMClusterControl(ocmClient)
	}

	return nil
}

func setInformer(k kubernetes.Interface, stopCh <-chan struct{}) error {
	secretInformer = corev1informer.NewSecretInformer(k, config.SecretNamespace, 0, cache.Indexers{
		cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
	})
	secretLister = corev1lister.NewSecretLister(secretInformer.GetIndexer())
	go secretInformer.Run(stopCh)
	return wait.PollImmediateUntil(time.Second, func() (done bool, err error) {
		return secretInformer.HasSynced(), nil
	}, stopCh)
}

// SetSecretControl is for test only
func SetSecretControl(ctrl cert.SecretControl) {
	secretControl = ctrl
}

// SetOCMClient is for test only
func SetOCMClient(c ocmclient.Interface) {
	ocmClient = c
}

// SetKubeClient is for test only
func SetKubeClient(k kubernetes.Interface) {
	kubeClient = k
}

func setOCMClusterInformer(c ocmclient.Interface, stopCh <-chan struct{}) error {
	ocmClusterInformers := clusterinformers.NewSharedInformerFactory(c, 0)
	clusterInformer = ocmClusterInformers.Cluster().V1().ManagedClusters().Informer()
	clusterLister = ocmClusterInformers.Cluster().V1().ManagedClusters().Lister()
	go clusterInformer.Run(stopCh)
	return wait.PollImmediateUntil(time.Second, func() (done bool, err error) {
		return clusterInformer.HasSynced(), nil
	}, stopCh)
}

func GetClusterControl() clusterutil.OCMClusterControl {
	return clusterControl
}
