package main

import (
	"k8s.io/klog"
	"sigs.k8s.io/apiserver-runtime/pkg/builder"

	// +kubebuilder:scaffold:resource-imports
	clusterv1 "github.com/oam-dev/cluster-gateway/pkg/apis/cluster/v1"

	"github.com/oam-dev/cluster-gateway/pkg/metrics"
)

func main() {

	// registering metrics
	metrics.Register()

	err := builder.APIServer.
		// +kubebuilder:scaffold:resource-register
		WithResource(&clusterv1.ClusterExtension{}).
		WithLocalDebugExtension().
		ExposeLoopbackClientConfig().
		ExposeLoopbackAuthorizer().
		Execute()
	if err != nil {
		klog.Fatal(err)
	}
}
