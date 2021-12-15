package multicluster

import "net/http"

var _ EnhanceClusterGatewayRoundTripper = &enhanceClusterGatewayRoundTripper{}

type EnhanceClusterGatewayRoundTripper interface {
	http.RoundTripper

	NewRoundTripper(delegate http.RoundTripper) http.RoundTripper
}

type enhanceClusterGatewayRoundTripper struct {
	clusterName string
	delegate    http.RoundTripper
}

func NewEnhanceClusterGatewayRoundTripper(clusterName string) EnhanceClusterGatewayRoundTripper {
	return &enhanceClusterGatewayRoundTripper{clusterName: clusterName}
}

func (e *enhanceClusterGatewayRoundTripper) NewRoundTripper(delegate http.RoundTripper) http.RoundTripper {
	e.delegate = delegate
	return e
}

func (e *enhanceClusterGatewayRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	request.URL.Path = formatProxyURL(e.clusterName, request.URL.Path)
	return e.delegate.RoundTrip(request)
}
