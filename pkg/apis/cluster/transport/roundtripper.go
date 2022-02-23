package multicluster

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/oam-dev/cluster-gateway/pkg/config"
)

var _ http.RoundTripper = &clusterGatewayRoundTripper{}

type clusterGatewayRoundTripper struct {
	delegate http.RoundTripper
	// falling back to the hosting cluster
	// this is required when the client does implicit api discovery
	// e.g. controller-runtime client
	fallback bool
}

func NewClusterGatewayRoundTripper(delegate http.RoundTripper) http.RoundTripper {
	return &clusterGatewayRoundTripper{
		delegate: delegate,
		fallback: true,
	}
}

func NewStrictClusterGatewayRoundTripper(delegate http.RoundTripper, fallback bool) http.RoundTripper {
	return &clusterGatewayRoundTripper{
		delegate: delegate,
		fallback: false,
	}
}

func (c *clusterGatewayRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	clusterName, exists := GetMultiClusterContext(request.Context())
	if !exists {
		if !c.fallback {
			return nil, fmt.Errorf("missing cluster name in the request context")
		}
		return c.delegate.RoundTrip(request)
	}
	request.URL.Path = formatProxyURL(clusterName, request.URL.Path)
	return c.delegate.RoundTrip(request)
}

func formatProxyURL(clusterName, originalPath string) string {
	originalPath = strings.TrimPrefix(originalPath, "/")
	return strings.Join([]string{
		"/apis",
		config.MetaApiGroupName,
		config.MetaApiVersionName,
		"clustergateways",
		clusterName,
		"proxy",
		originalPath}, "/")
}
