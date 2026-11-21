package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/pflag"
)

var (
	// ClusterGatewayTLSSecret is the [namespace/]name of the Secret to store the CA and serving certificate.
	// When non-empty, cluster-gateway will manage its own serving certificates and APIService caBundle.
	ClusterGatewayTLSSecret string

	// ClusterGatewayService is the [namespace/]name of the Service backing cluster-gateway.
	// If omitted, it is inferred from the target APIService.
	ClusterGatewayService string

	// ClusterGatewayAPIService is the name of the APIService whose caBundle is reconciled.
	ClusterGatewayAPIService = "v1alpha1.cluster.core.oam.dev"

	// ClusterGatewayCertDir is the local directory where serving cert files (tls.crt, tls.key) are written.
	ClusterGatewayCertDir string

	// ClusterGatewayTLSResyncPeriod is the interval between background renewal and caBundle repair cycles.
	ClusterGatewayTLSResyncPeriod = time.Minute
)

// ParseNamespacedName parses a string formatted as [namespace/]name.
func ParseNamespacedName(val, defaultNamespace string) (string, string, error) {
	val = strings.TrimSpace(val)
	if len(val) == 0 {
		return "", "", fmt.Errorf("name cannot be empty")
	}
	parts := strings.Split(val, "/")
	switch len(parts) {
	case 1:
		if len(defaultNamespace) == 0 {
			return "", "", fmt.Errorf("no namespace specified for %q and no default namespace provided", val)
		}
		return defaultNamespace, parts[0], nil
	case 2:
		if len(parts[0]) == 0 || len(parts[1]) == 0 {
			return "", "", fmt.Errorf("invalid [namespace/]name format: %q", val)
		}
		return parts[0], parts[1], nil
	default:
		return "", "", fmt.Errorf("invalid [namespace/]name format with multiple slashes: %q", val)
	}
}

// ValidateServingCert validates serving cert flags if enabled.
func ValidateServingCert() error {
	if len(ClusterGatewayTLSSecret) == 0 {
		if len(ClusterGatewayService) > 0 {
			return fmt.Errorf("--cluster-gateway-service can only be set when --cluster-gateway-tls-secret is configured")
		}
		return nil
	}

	if _, _, err := ParseNamespacedName(ClusterGatewayTLSSecret, SecretNamespace); err != nil {
		return fmt.Errorf("invalid --cluster-gateway-tls-secret: %w", err)
	}
	if len(ClusterGatewayService) > 0 {
		if _, _, err := ParseNamespacedName(ClusterGatewayService, SecretNamespace); err != nil {
			return fmt.Errorf("invalid --cluster-gateway-service: %w", err)
		}
	}
	if ClusterGatewayTLSResyncPeriod <= 0 {
		return fmt.Errorf("--cluster-gateway-tls-resync-period must be greater than 0")
	}
	return nil
}

// AddServingCertFlags registers the serving certificate management flags.
func AddServingCertFlags(set *pflag.FlagSet) {
	set.StringVar(&ClusterGatewayTLSSecret, "cluster-gateway-tls-secret", ClusterGatewayTLSSecret,
		"The [namespace/]name of the Secret to hold the serving cert and CA. When set, "+
			"cluster-gateway manages its own serving certificates and APIService caBundle.")
	set.StringVar(&ClusterGatewayService, "cluster-gateway-service", ClusterGatewayService,
		"The [namespace/]name of the Service pointing to cluster-gateway. Optional; "+
			"defaults to the Service specified on the target APIService.")
	set.StringVar(&ClusterGatewayAPIService, "cluster-gateway-apiservice", ClusterGatewayAPIService,
		"The APIService name whose caBundle is reconciled.")
	set.StringVar(&ClusterGatewayCertDir, "cluster-gateway-cert-dir", ClusterGatewayCertDir,
		"The local directory where cluster-gateway serving certs are written. Defaults to a temporary directory.")
	set.DurationVar(&ClusterGatewayTLSResyncPeriod, "cluster-gateway-tls-resync-period", ClusterGatewayTLSResyncPeriod,
		"The interval between serving cert renewal and APIService caBundle repair cycles.")
}
