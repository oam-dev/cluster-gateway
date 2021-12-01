package framework

import (
	"flag"
	"os"

	"k8s.io/klog/v2"
)

var context = &E2EContext{}

type E2EContext struct {
	HubKubeConfig string
	TestCluster   string
}

func ParseFlags() {
	registerFlags()
	flag.Parse()
	validateFlags()
}

func registerFlags() {
	flag.StringVar(&context.HubKubeConfig,
		"hub-kubeconfig",
		os.Getenv("KUBECONFIG"),
		"Path to kubeconfig of the hub cluster.")
	flag.StringVar(&context.TestCluster,
		"test-cluster",
		"",
		"The target cluster to run the e2e suite.")
}

func validateFlags() {
	if len(context.HubKubeConfig) == 0 {
		klog.Fatalf("--hub-kubeconfig is required")
	}
	if len(context.TestCluster) == 0 {
		klog.Fatalf("--test-cluster is required")
	}
}
