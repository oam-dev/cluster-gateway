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
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/rest"
	"k8s.io/utils/pointer"
)

func TestExchangeIdentity(t *testing.T) {
	testcases := map[string]struct {
		Exchanger *ClientIdentityExchanger
		UserInfo  user.Info
		Cluster   string
		Matched   bool
		RuleName  string
		Projected *rest.ImpersonationConfig
		Error     error
	}{
		"match-user": {
			Exchanger: &ClientIdentityExchanger{Rules: []ClientIdentityExchangeRule{{
				Name:   "user-match",
				Type:   PrivilegedIdentityExchanger,
				Source: &IdentityExchangerSource{User: pointer.String("test")},
			}}},
			UserInfo:  &user.DefaultInfo{Name: "test"},
			Matched:   true,
			RuleName:  "user-match",
			Projected: &rest.ImpersonationConfig{},
			Error:     nil,
		},
		"match-group": {
			Exchanger: &ClientIdentityExchanger{Rules: []ClientIdentityExchangeRule{{
				Name:   "group-match",
				Type:   StaticMappingIdentityExchanger,
				Source: &IdentityExchangerSource{Group: pointer.String("test-group")},
				Target: &IdentityExchangerTarget{Groups: []string{"projected"}},
			}}},
			UserInfo:  &user.DefaultInfo{Name: "test", Groups: []string{"group", "test-group"}},
			Matched:   true,
			RuleName:  "group-match",
			Projected: &rest.ImpersonationConfig{Groups: []string{"projected"}},
			Error:     nil,
		},
		"match-uid": {
			Exchanger: &ClientIdentityExchanger{Rules: []ClientIdentityExchangeRule{{
				Name:   "uid-match",
				Type:   PrivilegedIdentityExchanger,
				Source: &IdentityExchangerSource{UID: pointer.String("12345")},
			}}},
			UserInfo:  &user.DefaultInfo{Name: "abc", UID: "12345"},
			Matched:   true,
			RuleName:  "uid-match",
			Projected: &rest.ImpersonationConfig{},
			Error:     nil,
		},
		"match-cluster": {
			Exchanger: &ClientIdentityExchanger{Rules: []ClientIdentityExchangeRule{{
				Name:   "name-match",
				Type:   PrivilegedIdentityExchanger,
				Source: &IdentityExchangerSource{User: pointer.String("test"), Cluster: pointer.String("c1")},
			}, {
				Name:   "group-match",
				Type:   StaticMappingIdentityExchanger,
				Source: &IdentityExchangerSource{Group: pointer.String("test-group"), Cluster: pointer.String("c2")},
				Target: &IdentityExchangerTarget{Groups: []string{"projected"}},
			}}},
			UserInfo:  &user.DefaultInfo{Name: "test", Groups: []string{"group", "test-group"}},
			Cluster:   "c2",
			Matched:   true,
			RuleName:  "group-match",
			Projected: &rest.ImpersonationConfig{Groups: []string{"projected"}},
			Error:     nil,
		},
		"match-user-pattern": {
			Exchanger: &ClientIdentityExchanger{Rules: []ClientIdentityExchangeRule{{
				Name:   "user-pattern-match",
				Type:   PrivilegedIdentityExchanger,
				Source: &IdentityExchangerSource{UserPattern: pointer.String("test-.*")},
			}}},
			UserInfo:  &user.DefaultInfo{Name: "test-1234"},
			Matched:   true,
			RuleName:  "user-pattern-match",
			Projected: &rest.ImpersonationConfig{},
			Error:     nil,
		},
		"match-group-pattern": {
			Exchanger: &ClientIdentityExchanger{Rules: []ClientIdentityExchangeRule{{
				Name:   "group-pattern-match",
				Type:   StaticMappingIdentityExchanger,
				Source: &IdentityExchangerSource{GroupPattern: pointer.String("test-group:.+")},
				Target: &IdentityExchangerTarget{Groups: []string{"projected"}},
			}}},
			UserInfo:  &user.DefaultInfo{Name: "test", Groups: []string{"group:1", "test-group:2"}},
			Matched:   true,
			RuleName:  "group-pattern-match",
			Projected: &rest.ImpersonationConfig{Groups: []string{"projected"}},
			Error:     nil,
		},
		"match-cluster-pattern": {
			Exchanger: &ClientIdentityExchanger{Rules: []ClientIdentityExchangeRule{{
				Name:   "cluster-pattern-match",
				Type:   StaticMappingIdentityExchanger,
				Source: &IdentityExchangerSource{ClusterPattern: pointer.String("cluster-\\d+")},
				Target: &IdentityExchangerTarget{User: "special"},
			}}},
			UserInfo:  &user.DefaultInfo{Name: "test"},
			Cluster:   "cluster-1",
			Matched:   true,
			RuleName:  "cluster-pattern-match",
			Projected: &rest.ImpersonationConfig{UserName: "special"},
			Error:     nil,
		},
		"not-implemented": {
			Exchanger: &ClientIdentityExchanger{Rules: []ClientIdentityExchangeRule{{
				Name:   "external-identity-exchange",
				Type:   ExternalIdentityExchanger,
				Source: &IdentityExchangerSource{ClusterPattern: pointer.String("cluster-\\d+")},
				URL:    pointer.String("http://1.2.3.4:5000"),
			}}},
			UserInfo:  &user.DefaultInfo{Name: "test"},
			Cluster:   "cluster-1",
			Matched:   true,
			RuleName:  "external-identity-exchange",
			Projected: nil,
			Error:     fmt.Errorf("ExternalIdentityExchanger is not implemented"),
		},
		"no-match": {
			Exchanger: &ClientIdentityExchanger{Rules: []ClientIdentityExchangeRule{{
				Name:   "cluster-pattern-match",
				Type:   StaticMappingIdentityExchanger,
				Source: &IdentityExchangerSource{ClusterPattern: pointer.String("cluster-\\d+")},
				Target: &IdentityExchangerTarget{User: "special"},
			}}},
			UserInfo: &user.DefaultInfo{Name: "test"},
			Cluster:  "cluster-other",
			Matched:  false,
		},
	}
	for name, tt := range testcases {
		t.Run(name, func(t *testing.T) {
			matched, ruleName, projected, err := ExchangeIdentity(tt.Exchanger, tt.UserInfo, tt.Cluster)
			if tt.Error != nil {
				require.Error(t, tt.Error, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.Matched, matched)
			require.Equal(t, tt.RuleName, ruleName)
			require.Equal(t, tt.Projected, projected)
		})
	}
}
