package framework

import (
	"flag"
	"os"
	"path/filepath"

	"k8s.io/klog/v2"
)

var context = &E2EContext{}

type E2EContext struct {
	HubKubeConfig  string
	TestCluster    string
	IsOCMInstalled bool
}

func ParseFlags() {
	registerFlags()
	flag.Parse()
	defaultFlags()
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
	flag.BoolVar(&context.IsOCMInstalled,
		"ocm-installed",
		false,
		"Is the test running inside OCM environment")
}

func defaultFlags() {
	if len(context.HubKubeConfig) == 0 {
		home := os.Getenv("HOME")
		if len(home) > 0 {
			context.HubKubeConfig = filepath.Join(home, ".kube", "config")
		}
	}
}

func validateFlags() {
	if len(context.HubKubeConfig) == 0 {
		klog.Fatalf("--hub-kubeconfig is required")
	}
	if len(context.TestCluster) == 0 {
		klog.Fatalf("--test-cluster is required")
	}
}
