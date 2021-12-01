package e2e

import (
	"testing"

	"github.com/onsi/ginkgo"
)

func RunE2ETests(t *testing.T) {
	ginkgo.RunSpecs(t, "ClusterGateway e2e suite")
}
