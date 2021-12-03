package config

import (
	"github.com/spf13/pflag"
)

var AuthorizateProxySubpath bool

func AddProxyAuthorizationFlags(set *pflag.FlagSet) {
	set.BoolVarP(&AuthorizateProxySubpath, "authorize-proxy-subpath", "", false,
		"perform an additional delegated authorization against the hub cluster for the target proxying path when invoking clustergateway/proxy subresource")
}
