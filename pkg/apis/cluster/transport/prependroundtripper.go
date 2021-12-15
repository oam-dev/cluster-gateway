package multicluster

import "net/http"

var _ ProxyPathPrependingClusterGatewayRoundTripper = &proxyPathPrependingClusterGatewayRoundTripper{}

type ProxyPathPrependingClusterGatewayRoundTripper interface {
	http.RoundTripper

	NewRoundTripper(delegate http.RoundTripper) http.RoundTripper
}

type proxyPathPrependingClusterGatewayRoundTripper struct {
	clusterName string
	delegate    http.RoundTripper
}

func NewProxyPathPrependingClusterGatewayRoundTripper(clusterName string) ProxyPathPrependingClusterGatewayRoundTripper {
	return &proxyPathPrependingClusterGatewayRoundTripper{clusterName: clusterName}
}

func (p *proxyPathPrependingClusterGatewayRoundTripper) NewRoundTripper(delegate http.RoundTripper) http.RoundTripper {
	p.delegate = delegate
	return p
}

func (p *proxyPathPrependingClusterGatewayRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	request.URL.Path = formatProxyURL(p.clusterName, request.URL.Path)
	return p.delegate.RoundTrip(request)
}
