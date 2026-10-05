// Package loopback holds the loopback client config and authorizer produced
// while bootstrapping the aggregated apiserver, so in-process consumers
// (e.g. the proxy subresource) can reach the hosting kube-apiserver without
// a second round of client bootstrapping.
package loopback

import (
	"sync"

	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/client-go/rest"
)

var (
	authzOnce sync.Once
	authz     authorizer.Authorizer

	masterClientConfigOnce sync.Once
	masterClientConfig     *rest.Config
)

// SetAuthorizer provides the loopback authorizer, once.
func SetAuthorizer(a authorizer.Authorizer) {
	authzOnce.Do(func() {
		authz = a
	})
}

// GetAuthorizer returns the loopback authorizer performing delegated authorization.
func GetAuthorizer() authorizer.Authorizer {
	return authz
}

// SetLoopbackMasterClientConfig provides the loopback client config for
// accessing the configured master cluster's kube-apiserver, once.
func SetLoopbackMasterClientConfig(c *rest.Config) {
	masterClientConfigOnce.Do(func() {
		masterClientConfig = c
	})
}

// GetLoopbackMasterClientConfig returns the loopback client config for the
// master kube-apiserver.
func GetLoopbackMasterClientConfig() *rest.Config {
	return masterClientConfig
}
