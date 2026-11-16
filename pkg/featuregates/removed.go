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
	"strconv"
	"strings"

	"k8s.io/klog/v2"
)

// removedAPFGate is the APIPriorityAndFairness feature gate, removed from Kubernetes once API Priority
// and Fairness went GA (it is now turned off with --enable-priority-and-fairness=false). Deployments
// written for older gateways, such as KubeVela's vela-core chart, still pass
// --feature-gates=APIPriorityAndFairness=false, and the apiserver exits on an unknown gate.
const removedAPFGate = "APIPriorityAndFairness"

// TranslateRemovedGates rewrites command-line args (program name first) so that a removed
// APIPriorityAndFairness=<bool> in --feature-gates is dropped and its value passed as
// --enable-priority-and-fairness=<bool>, unless that flag is already given.
func TranslateRemovedGates(args []string) []string {
	if len(args) == 0 {
		return args
	}
	out := []string{args[0]}
	var rest []string
	apf, apfFlagGiven := "", false
	for i := 1; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			rest = args[i:]
			break
		}
		name, value, inline := strings.Cut(arg, "=")
		if name == "--enable-priority-and-fairness" {
			apfFlagGiven = true
		}
		if name != "--feature-gates" {
			out = append(out, arg)
			continue
		}
		if !inline {
			if i+1 == len(args) {
				out = append(out, arg)
				continue
			}
			i++
			value = args[i]
		}
		kept, v, found := dropRemovedGate(value)
		if found {
			apf = v
		}
		if kept != "" {
			out = append(out, "--feature-gates="+kept)
		}
	}
	if apf != "" {
		if apfFlagGiven {
			klog.Warningf("ignoring --feature-gates=%s=%s: the gate was removed from Kubernetes; --enable-priority-and-fairness is set", removedAPFGate, apf)
		} else {
			klog.Warningf("--feature-gates=%s=%s is no longer supported (the gate was removed from Kubernetes); using --enable-priority-and-fairness=%s", removedAPFGate, apf, apf)
			out = append(out, "--enable-priority-and-fairness="+apf)
		}
	}
	return append(out, rest...)
}

// dropRemovedGate removes the removed gate from a --feature-gates value ("A=true,B=false", where a
// key may carry a component prefix such as "kube:") and returns the remaining value and the gate's
// value. Only a gate with a boolean value is removed: anything else (no value, "maybe") is kept, so
// the feature-gate parser rejects it as before.
func dropRemovedGate(gates string) (kept string, value string, found bool) {
	var keep []string
	for _, kv := range strings.Split(gates, ",") {
		key, v, _ := strings.Cut(strings.TrimSpace(kv), "=")
		if i := strings.LastIndex(key, ":"); i >= 0 {
			key = key[i+1:]
		}
		if b, err := strconv.ParseBool(strings.TrimSpace(v)); key == removedAPFGate && err == nil {
			value, found = strconv.FormatBool(b), true
			continue
		}
		if strings.TrimSpace(kv) != "" {
			keep = append(keep, strings.TrimSpace(kv))
		}
	}
	return strings.Join(keep, ","), value, found
}
