package config

import (
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
)

var ClusterProxyHost string
var ClusterProxyPort int
var ClusterProxyCAFile string
var ClusterProxyCertFile string
var ClusterProxyKeyFile string

func ValidateClusterProxy() error {
	if len(ClusterProxyHost) == 0 {
		return nil
	}
	if ClusterProxyPort == 0 {
		return errors.New("--proxy-port must be greater than 0")
	}
	if len(ClusterProxyCAFile) == 0 {
		return errors.New("--proxy-ca-cert must be specified")
	}
	if len(ClusterProxyCertFile) == 0 {
		return errors.New("--proxy-cert must be specified")
	}
	if len(ClusterProxyKeyFile) == 0 {
		return errors.New("--proxy-key must be specified")
	}
	return nil
}

func AddClusterProxyFlags(set *pflag.FlagSet) {
	set.StringVarP(&ClusterProxyHost, "proxy-host", "", "",
		"the host of the cluster proxy endpoint")
	set.IntVarP(&ClusterProxyPort, "proxy-port", "", 8090,
		"the port of the cluster proxy endpoint")
	set.StringVarP(&ClusterProxyCAFile, "proxy-ca-cert", "", "",
		"the path to ca file for connecting cluster proxy")
	set.StringVarP(&ClusterProxyCertFile, "proxy-cert", "", "",
		"the path to tls cert for connecting cluster proxy")
	set.StringVarP(&ClusterProxyKeyFile, "proxy-key", "", "",
		"the path to tls key for connecting cluster proxy")
}
