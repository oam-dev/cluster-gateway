package featuregates

import (
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/component-base/featuregate"
	"k8s.io/klog/v2"
)

func init() {
	if err := utilfeature.DefaultMutableFeatureGate.Add(DefaultKubeFedFeatureGates); err != nil {
		klog.Fatalf("Unexpected error: %v", err)
	}
}

const (
	// owner: @yue9944882
	// alpha: v1.1.12
	//
	// HealthinessCheck enables the "/health" subresource on the ClusterGateway
	// by which we can read/update the healthiness related status under the
	// ".status".
	//
	// Additionally, OCM cluster-gateway addon will enable a health-check controller
	// in the background which periodically checks the healthiness for each managed
	// cluster by dialing "/healthz" api path.
	HealthinessCheck featuregate.Feature = "HealthinessCheck"

	// owner: @yue9944882
	// alpha: v1.1.15
	//
	// SecretCache runs a namespaced secret informer inside the apiserver which
	// provides a cache for reading secret data.
	SecretCache featuregate.Feature = "SecretCache"
)

var DefaultKubeFedFeatureGates = map[featuregate.Feature]featuregate.FeatureSpec{
	HealthinessCheck: {Default: false, PreRelease: featuregate.Alpha},
	SecretCache:      {Default: false, PreRelease: featuregate.Alpha},
}
