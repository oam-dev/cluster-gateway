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
	"github.com/oam-dev/cluster-gateway/pkg/util/singleton"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/server"
	"k8s.io/klog/v2"
	"net/http"
	"sigs.k8s.io/apiserver-runtime/pkg/builder"
	// +kubebuilder:scaffold:resource-imports
	clusterv1alpha1 "github.com/oam-dev/cluster-gateway/pkg/apis/cluster/v1alpha1"
	genericfilters "k8s.io/apiserver/pkg/server/filters"

	_ "github.com/oam-dev/cluster-gateway/pkg/featuregates"
)

func main() {

	// registering metrics
	metrics.Register()

	cmd, err := builder.APIServer.
		// +kubebuilder:scaffold:resource-register
		WithResource(&clusterv1alpha1.ClusterGateway{}).
		WithLocalDebugExtension().
		ExposeLoopbackMasterClientConfig().
		ExposeLoopbackAuthorizer().
		WithoutEtcd().
		WithConfigFns(func(config *server.RecommendedConfig) *server.RecommendedConfig {
			config.LongRunningFunc = func(r *http.Request, requestInfo *request.RequestInfo) bool {
				if requestInfo.Resource == "clustergateways" && requestInfo.Subresource == "proxy" {
					return true
				}
				return genericfilters.BasicLongRunningRequestCheck(sets.NewString("watch"), sets.NewString())(r, requestInfo)
			}
			return config
		}).
		WithOptionsFns(func(options *builder.ServerOptions) *builder.ServerOptions {
			if err := config.ValidateSecret(); err != nil {
				klog.Fatal(err)
			}
			if err := config.ValidateClusterProxy(); err != nil {
				klog.Fatal(err)
			}
			return options
		}).
		WithPostStartHook("init-master-loopback-client", singleton.InitLoopbackClient).
		Build()
	if err != nil {
		klog.Fatal(err)
	}
	config.AddSecretFlags(cmd.Flags())
	config.AddClusterProxyFlags(cmd.Flags())
	config.AddProxyAuthorizationFlags(cmd.Flags())
	cmd.Flags().BoolVarP(&options.OCMIntegration, "ocm-integration", "", false,
		"Enabling OCM integration, reading cluster CA and api endpoint from managed "+
			"cluster.")
	if err := cmd.Execute(); err != nil {
		klog.Fatal(err)
	}
}
