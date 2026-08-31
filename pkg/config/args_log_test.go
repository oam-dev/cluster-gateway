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
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddLogFlags(t *testing.T) {
	set := pflag.NewFlagSet("test", pflag.ContinueOnError)
	AddLogFlags(set)

	// klog flags should be registered
	f := set.Lookup("v")
	assert.NotNil(t, f, "expected klog -v flag to be registered")

	// legacy_stderr_threshold_behavior should be opted out
	legacy := set.Lookup("legacy_stderr_threshold_behavior")
	require.NotNil(t, legacy, "expected legacy_stderr_threshold_behavior flag")
	assert.Equal(t, "false", legacy.Value.String())

	// stderrthreshold should default to INFO
	threshold := set.Lookup("stderrthreshold")
	require.NotNil(t, threshold, "expected stderrthreshold flag")
	// klog maps INFO to severity 0
	assert.Equal(t, "0", threshold.Value.String())
}
