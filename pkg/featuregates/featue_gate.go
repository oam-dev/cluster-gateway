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
	// HealthinessCheck
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

	// SecretCache
	// owner: @yue9944882
	// alpha: v1.1.15
	//
	// SecretCache runs a namespaced secret informer inside the apiserver which
	// provides a cache for reading secret data.
	SecretCache featuregate.Feature = "SecretCache"

	// OCMClusterCache
	// owner: @ivan-cai
	// beta: v1.6.0
	//
	// SecretCache runs a OCM ManagedCluster informer inside the apiserver which
	// provides a cache for reading ManagedCluster.
	OCMClusterCache featuregate.Feature = "OCMClusterCache"

	// ClientIdentityPenetration
	// owner: @somefive
	// alpha: v1.4.0
	//
	// ClientIdentityPenetration enforce impersonate as the original request user
	// when accessing apiserver in ManagedCluster
	ClientIdentityPenetration featuregate.Feature = "ClientIdentityPenetration"

	// VirtualCluster
	// owner: @somefive
	// alpha: v1.9.0
	//
	// VirtualCluster add virtual cluster api to access managed cluster metadata
	VirtualCluster featuregate.Feature = "VirtualCluster"
)

var DefaultKubeFedFeatureGates = map[featuregate.Feature]featuregate.FeatureSpec{
	HealthinessCheck:          {Default: false, PreRelease: featuregate.Beta},
	SecretCache:               {Default: true, PreRelease: featuregate.Beta},
	OCMClusterCache:           {Default: true, PreRelease: featuregate.Beta},
	ClientIdentityPenetration: {Default: false, PreRelease: featuregate.Alpha},
	VirtualCluster:            {Default: false, PreRelease: featuregate.Alpha},
}
