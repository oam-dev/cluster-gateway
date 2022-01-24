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
	// alpha: v0.1
	//
	// Accessing clusters via Apiserver-Network-Proxy.
	// https://github.com/kubernetes-sigs/apiserver-network-proxy
	HealthinessCheck featuregate.Feature = "HealthinessCheck"
)

var DefaultKubeFedFeatureGates = map[featuregate.Feature]featuregate.FeatureSpec{
	HealthinessCheck: {Default: false, PreRelease: featuregate.Alpha},
}
