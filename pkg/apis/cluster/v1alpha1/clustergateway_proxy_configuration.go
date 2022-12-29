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

package v1alpha1

import (
	"fmt"
	"os"
	"regexp"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/rest"
	"k8s.io/utils/strings/slices"

	"github.com/oam-dev/cluster-gateway/pkg/config"
)

const (
	AnnotationClusterGatewayProxyConfiguration = "cluster.core.oam.dev/cluster-gateway-proxy-configuration"
)

type ClusterGatewayProxyConfiguration struct {
	metav1.TypeMeta `json:",inline"`
	Spec            ClusterGatewayProxyConfigurationSpec `json:"spec"`
}

type ClusterGatewayProxyConfigurationSpec struct {
	ClientIdentityExchanger `json:"clientIdentityExchanger"`
}

type ClientIdentityExchanger struct {
	Rules []ClientIdentityExchangeRule `json:"rules,omitempty"`
}

type ClientIdentityExchangeType string

const (
	PrivilegedIdentityExchanger    ClientIdentityExchangeType = "PrivilegedIdentityExchanger"
	StaticMappingIdentityExchanger ClientIdentityExchangeType = "StaticMappingIdentityExchanger"
	ExternalIdentityExchanger      ClientIdentityExchangeType = "ExternalIdentityExchanger"
)

type ClientIdentityExchangeRule struct {
	Name   string                     `json:"name"`
	Type   ClientIdentityExchangeType `json:"type"`
	Source *IdentityExchangerSource   `json:"source"`

	Target *IdentityExchangerTarget `json:"target,omitempty"`
	URL    *string                  `json:"url,omitempty"`
}

type IdentityExchangerTarget struct {
	User   string   `json:"user,omitempty"`
	Groups []string `json:"groups,omitempty"`
	UID    string   `json:"uid,omitempty"`
}

type IdentityExchangerSource struct {
	User    *string `json:"user,omitempty"`
	Group   *string `json:"group,omitempty"`
	UID     *string `json:"uid,omitempty"`
	Cluster *string `json:"cluster,omitempty"`

	UserPattern    *string `json:"userPattern,omitempty"`
	GroupPattern   *string `json:"groupPattern,omitempty"`
	ClusterPattern *string `json:"clusterPattern,omitempty"`
}

var GlobalClusterGatewayProxyConfiguration = &ClusterGatewayProxyConfiguration{}

func LoadGlobalClusterGatewayProxyConfig() error {
	if config.ClusterGatewayProxyConfigPath == "" {
		return nil
	}
	bs, err := os.ReadFile(config.ClusterGatewayProxyConfigPath)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(bs, GlobalClusterGatewayProxyConfiguration)
}

func ExchangeIdentity(exchanger *ClientIdentityExchanger, userInfo user.Info, cluster string) (matched bool, ruleName string, projected *rest.ImpersonationConfig, err error) {
	for _, rule := range exchanger.Rules {
		if matched, projected, err = exchangeIdentity(&rule, userInfo, cluster); matched {
			return matched, rule.Name, projected, err
		}
	}
	return false, "", nil, nil
}

func exchangeIdentity(rule *ClientIdentityExchangeRule, userInfo user.Info, cluster string) (matched bool, projected *rest.ImpersonationConfig, err error) {
	if !matchIdentity(rule.Source, userInfo, cluster) {
		return false, nil, nil
	}
	switch rule.Type {
	case PrivilegedIdentityExchanger:
		return true, &rest.ImpersonationConfig{}, nil
	case StaticMappingIdentityExchanger:
		return true, &rest.ImpersonationConfig{
			UserName: rule.Target.User,
			Groups:   rule.Target.Groups,
			UID:      rule.Target.UID,
		}, nil
	case ExternalIdentityExchanger:
		return true, nil, fmt.Errorf("ExternalIdentityExchanger is not implemented")
	}
	return true, nil, fmt.Errorf("unknown exchanger type: %s", rule.Type)
}

// denyQuery return true when the pattern is valid and could be used as regular expression,
// and the given query does not match the pattern, otherwise return false
func (in *IdentityExchangerSource) denyQuery(pattern *string, query string) bool {
	if pattern == nil {
		return false
	}
	matched, err := regexp.Match(*pattern, []byte(query))
	if err != nil {
		return false
	}
	return !matched
}

// denyGroups return true if none of the group matches the given pattern
func (in *IdentityExchangerSource) denyGroups(groupPattern *string, groups []string) bool {
	if groupPattern == nil {
		return false
	}
	for _, group := range groups {
		if !in.denyQuery(groupPattern, group) {
			return false
		}
	}
	return true
}

func matchIdentity(in *IdentityExchangerSource, userInfo user.Info, cluster string) bool {
	if in == nil {
		return false
	}
	switch {
	case in.User != nil && userInfo.GetName() != *in.User:
		return false
	case in.Group != nil && !slices.Contains(userInfo.GetGroups(), *in.Group):
		return false
	case in.UID != nil && userInfo.GetUID() != *in.UID:
		return false
	case in.Cluster != nil && cluster != *in.Cluster:
		return false
	case in.denyQuery(in.UserPattern, userInfo.GetName()):
		return false
	case in.denyGroups(in.GroupPattern, userInfo.GetGroups()):
		return false
	case in.denyQuery(in.ClusterPattern, cluster):
		return false
	}
	return true
}
