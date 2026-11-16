/*
Copyright 2026 The KubeVela Authors.

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

package featuregates

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTranslateRemovedGates(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "KubeVela's vela-core chart",
			in:   []string{"cluster-gateway", "apiserver", "--secure-port=9443", "--feature-gates=APIPriorityAndFairness=false,ClientIdentityPenetration=false", "--tls-cert-file=/c/tls.crt"},
			want: []string{"cluster-gateway", "apiserver", "--secure-port=9443", "--feature-gates=ClientIdentityPenetration=false", "--tls-cert-file=/c/tls.crt", "--enable-priority-and-fairness=false"},
		},
		{
			name: "the removed gate alone",
			in:   []string{"cluster-gateway", "--feature-gates=APIPriorityAndFairness=false"},
			want: []string{"cluster-gateway", "--enable-priority-and-fairness=false"},
		},
		{
			name: "value as the next arg, gate true",
			in:   []string{"cluster-gateway", "--feature-gates", "APIPriorityAndFairness=true,HealthinessCheck=true"},
			want: []string{"cluster-gateway", "--feature-gates=HealthinessCheck=true", "--enable-priority-and-fairness=true"},
		},
		{
			name: "component prefix",
			in:   []string{"cluster-gateway", "--feature-gates=kube:APIPriorityAndFairness=false"},
			want: []string{"cluster-gateway", "--enable-priority-and-fairness=false"},
		},
		{
			name: "the flag wins over the removed gate",
			in:   []string{"cluster-gateway", "--enable-priority-and-fairness=true", "--feature-gates=APIPriorityAndFairness=false,ClientIdentityPenetration=true"},
			want: []string{"cluster-gateway", "--enable-priority-and-fairness=true", "--feature-gates=ClientIdentityPenetration=true"},
		},
		{
			name: "no removed gate: unchanged",
			in:   []string{"cluster-gateway", "--feature-gates=ClientIdentityPenetration=true", "--secure-port=9443"},
			want: []string{"cluster-gateway", "--feature-gates=ClientIdentityPenetration=true", "--secure-port=9443"},
		},
		{
			name: "args after -- stay last",
			in:   []string{"cluster-gateway", "--feature-gates=APIPriorityAndFairness=false", "--", "--feature-gates=APIPriorityAndFairness=false"},
			want: []string{"cluster-gateway", "--enable-priority-and-fairness=false", "--", "--feature-gates=APIPriorityAndFairness=false"},
		},
		{
			name: "--feature-gates without a value: unchanged",
			in:   []string{"cluster-gateway", "--feature-gates"},
			want: []string{"cluster-gateway", "--feature-gates"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, TranslateRemovedGates(c.in))
		})
	}
}
