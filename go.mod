module github.com/oam-dev/cluster-gateway

go 1.16

require (
	github.com/ghodss/yaml v1.0.0
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.17.0
	github.com/openshift/api v0.0.0-20210924154557-a4f696157341 // indirect
	github.com/openshift/library-go v0.0.0-20210916194400-ae21aab32431
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.2.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	golang.org/x/crypto v0.0.0-20211202192323-5770296d904e // indirect
	google.golang.org/grpc v1.40.0
	k8s.io/api v0.22.4
	k8s.io/apimachinery v0.22.4
	k8s.io/apiserver v0.22.4
	k8s.io/client-go v0.22.4
	k8s.io/component-base v0.22.4
	k8s.io/klog v1.0.0
	k8s.io/klog/v2 v2.9.0
	k8s.io/kube-aggregator v0.22.4
	k8s.io/utils v0.0.0-20210930125809-cb0fa318a74b
	open-cluster-management.io/addon-framework v0.1.1-0.20220117030117-10147aa1fdcf
	open-cluster-management.io/api v0.5.1-0.20220112073018-2d280a97a052
	open-cluster-management.io/managed-serviceaccount v0.1.0
	sigs.k8s.io/apiserver-network-proxy v0.0.24
	sigs.k8s.io/apiserver-network-proxy/konnectivity-client v0.0.25
	sigs.k8s.io/apiserver-runtime v1.0.3-0.20210913073608-0663f60bfee2
	sigs.k8s.io/controller-runtime v0.9.5
)

replace sigs.k8s.io/apiserver-network-proxy/konnectivity-client => sigs.k8s.io/apiserver-network-proxy/konnectivity-client v0.0.24
