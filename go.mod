module github.com/oam-dev/cluster-gateway

go 1.16

require (
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.14.0
	github.com/openshift/library-go v0.0.0-20210916194400-ae21aab32431
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	golang.org/x/crypto v0.0.0-20210322153248-0c34fe9e7dc2 // indirect
	google.golang.org/grpc v1.38.0
	k8s.io/api v0.22.1
	k8s.io/apimachinery v0.22.1
	k8s.io/apiserver v0.22.1
	k8s.io/client-go v0.22.1
	k8s.io/component-base v0.22.1
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.9.0
	k8s.io/kube-aggregator v0.22.1
	k8s.io/utils v0.0.0-20210802155522-efc7438f0176
	open-cluster-management.io/addon-framework v0.1.1-0.20211214072539-e51264d353a2
	open-cluster-management.io/api v0.5.0
	open-cluster-management.io/managed-serviceaccount v0.1.0
	sigs.k8s.io/apiserver-network-proxy v0.0.24
	sigs.k8s.io/apiserver-network-proxy/konnectivity-client v0.0.24
	sigs.k8s.io/apiserver-runtime v1.0.3-0.20210913073608-0663f60bfee2
	sigs.k8s.io/controller-runtime v0.9.5
)

replace sigs.k8s.io/apiserver-network-proxy/konnectivity-client => sigs.k8s.io/apiserver-network-proxy/konnectivity-client v0.0.24
