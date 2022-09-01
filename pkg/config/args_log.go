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
	"flag"

	"github.com/spf13/pflag"
	"k8s.io/klog/v2"
)

// AddLogFlags add log flags to command
func AddLogFlags(set *pflag.FlagSet) {
	fs := flag.NewFlagSet("", 0)
	klog.InitFlags(fs)
	set.AddGoFlagSet(fs)
}
