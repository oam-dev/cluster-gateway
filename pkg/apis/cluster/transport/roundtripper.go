package multicluster

import (
	"fmt"
	"net/http"
	"strings"

	clusterapi "github.com/oam-dev/cluster-gateway/pkg/apis/cluster/v1alpha1"
)

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

var _ http.RoundTripper = &clusterGatewayRoundTripper{}

type clusterGatewayRoundTripper struct {
	delegate http.RoundTripper
	// falling back to the hosting cluster
	// this is required when the client does implicit api discovery
	// e.g. controller-runtime client
	fallback bool
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
		clusterapi.SchemeGroupVersion.Group,
		clusterapi.SchemeGroupVersion.Version,
		"clustergateways",
		clusterName,
		"proxy",
		originalPath}, "/")
}
