package config

import (
	"github.com/spf13/pflag"
)

var ProxyLocalAuthorization bool

func AddProxyAuthorizationFlags(set *pflag.FlagSet) {
	set.BoolVarP(&ProxyLocalAuthorization, "proxy-local-authorization", "", false,
		"do authorization locally before proxy request")
}
