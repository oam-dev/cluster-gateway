/*
Copyright 2021 The KubeVela Authors.

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

package main

import (
	"github.com/oam-dev/cluster-gateway/pkg/config"
	"github.com/oam-dev/cluster-gateway/pkg/metrics"
	"github.com/oam-dev/cluster-gateway/pkg/options"

	"k8s.io/klog"
	"sigs.k8s.io/apiserver-runtime/pkg/builder"

	// +kubebuilder:scaffold:resource-imports
	clusterv1alpha1 "github.com/oam-dev/cluster-gateway/pkg/apis/cluster/v1alpha1"
)

func main() {

	// registering metrics
	metrics.Register()

	cmd, err := builder.APIServer.
		// +kubebuilder:scaffold:resource-register
		WithResource(&clusterv1alpha1.ClusterGateway{}).
		WithLocalDebugExtension().
		DisableAuthorization().
		ExposeLoopbackMasterClientConfig().
		WithoutEtcd().
		WithOptionsFns(func(options *builder.ServerOptions) *builder.ServerOptions {
			if err := config.Validate(); err != nil {
				panic(err)
			}
			return options
		}).
		Build()
	if err != nil {
		klog.Fatal(err)
	}
	config.AddFlags(cmd.Flags())
	cmd.Flags().BoolVarP(
		&options.OCMIntegration, "ocm-integration", "", false,
		"Enabling OCM integration, reading cluster CA and api endpoint from managed "+
			"cluster.")
	if err := cmd.Execute(); err != nil {
		klog.Fatal(err)
	}
}
