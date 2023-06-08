package roundtrip

import (
	"os"
	"testing"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/oam-dev/cluster-gateway/e2e/framework"
)

func TestMain(m *testing.M) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	framework.ParseFlags()
	os.Exit(m.Run())
}

func RunE2ETests(t *testing.T) {
	ginkgo.RunSpecs(t, "ClusterGateway e2e suite -- basic api round-tripping")
}

func TestE2E(t *testing.T) {
	RunE2ETests(t)
}
