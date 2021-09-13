// +build !secret

package config

import "github.com/spf13/pflag"

func Validate() error {
	return nil
}

func AddFlags(set *pflag.FlagSet) {
}
