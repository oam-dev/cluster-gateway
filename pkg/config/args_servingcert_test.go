package config

import (
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

func TestParseNamespacedName(t *testing.T) {
	// Full namespace/name
	ns, name, err := ParseNamespacedName("vela-system/my-secret", "default")
	require.NoError(t, err)
	require.Equal(t, "vela-system", ns)
	require.Equal(t, "my-secret", name)

	// Name only with default namespace
	ns, name, err = ParseNamespacedName("my-secret", "default")
	require.NoError(t, err)
	require.Equal(t, "default", ns)
	require.Equal(t, "my-secret", name)

	// Empty name
	_, _, err = ParseNamespacedName("", "default")
	require.Error(t, err)

	// Multiple slashes
	_, _, err = ParseNamespacedName("ns/sub/name", "default")
	require.Error(t, err)

	// Missing namespace and no default
	_, _, err = ParseNamespacedName("my-secret", "")
	require.Error(t, err)

	// Slash with empty name or ns
	_, _, err = ParseNamespacedName("/name", "default")
	require.Error(t, err)
	_, _, err = ParseNamespacedName("ns/", "default")
	require.Error(t, err)
}

func TestValidateServingCert(t *testing.T) {
	origSecret := ClusterGatewayTLSSecret
	origService := ClusterGatewayService
	origPeriod := ClusterGatewayTLSResyncPeriod
	t.Cleanup(func() {
		ClusterGatewayTLSSecret = origSecret
		ClusterGatewayService = origService
		ClusterGatewayTLSResyncPeriod = origPeriod
	})

	ClusterGatewayTLSSecret = ""
	ClusterGatewayService = ""
	ClusterGatewayTLSResyncPeriod = time.Minute
	require.NoError(t, ValidateServingCert())

	// Standalone service without secret is rejected
	ClusterGatewayTLSSecret = ""
	ClusterGatewayService = "vela-system/my-service"
	require.Error(t, ValidateServingCert())

	ClusterGatewayTLSSecret = "vela-system/my-secret"
	ClusterGatewayService = "vela-system/my-service"
	require.NoError(t, ValidateServingCert())

	ClusterGatewayTLSSecret = "a/b/c"
	require.Error(t, ValidateServingCert())

	ClusterGatewayTLSSecret = "vela-system/my-secret"
	ClusterGatewayService = "a/b/c"
	require.Error(t, ValidateServingCert())

	ClusterGatewayTLSSecret = "vela-system/my-secret"
	ClusterGatewayService = ""
	ClusterGatewayTLSResyncPeriod = 0
	require.Error(t, ValidateServingCert())
	ClusterGatewayTLSResyncPeriod = -1 * time.Second
	require.Error(t, ValidateServingCert())
}

func TestAddServingCertFlags(t *testing.T) {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	AddServingCertFlags(fs)
	require.NotNil(t, fs.Lookup("cluster-gateway-tls-secret"))
	require.NotNil(t, fs.Lookup("cluster-gateway-service"))
	require.NotNil(t, fs.Lookup("cluster-gateway-apiservice"))
	require.NotNil(t, fs.Lookup("cluster-gateway-cert-dir"))
	require.NotNil(t, fs.Lookup("cluster-gateway-tls-resync-period"))
}
