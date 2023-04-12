package config

import (
	"fmt"

	"github.com/spf13/pflag"
)

// SecretNamespace the namespace to search cluster credentials
var SecretNamespace = "vela-system"

func ValidateSecret() error {
	if len(SecretNamespace) == 0 {
		return fmt.Errorf("must specify --secret-namespace")
	}
	return nil
}

func AddSecretFlags(set *pflag.FlagSet) {
	set.StringVarP(&SecretNamespace, "secret-namespace", "", SecretNamespace,
		"the namespace to reading secrets")
}
