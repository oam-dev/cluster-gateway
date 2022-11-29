/*
Copyright 2022 The KubeVela Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package config

import (
	"github.com/spf13/pflag"
	"k8s.io/apiserver/pkg/server"
)

var UserAgent string

func AddUserAgentFlags(set *pflag.FlagSet) {
	set.StringVarP(&UserAgent, "user-agent", "", "",
		"Specifying the UserAgent for communicating with the host cluster.")
}

func WithUserAgent(config *server.RecommendedConfig) *server.RecommendedConfig {
	if UserAgent != "" {
		config.ClientConfig.UserAgent = UserAgent
	}
	return config
}
