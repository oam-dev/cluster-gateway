package framework

import (
	clusterv1alpha1 "github.com/oam-dev/cluster-gateway/pkg/apis/cluster/v1alpha1"
	proxyv1alpha1 "github.com/oam-dev/cluster-gateway/pkg/apis/proxy/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
)

var scheme = runtime.NewScheme()

func init() {
	clusterv1alpha1.AddToScheme(scheme)
	proxyv1alpha1.AddToScheme(scheme)
	clusterv1.AddToScheme(scheme)
	addonv1alpha1.AddToScheme(scheme)
}
