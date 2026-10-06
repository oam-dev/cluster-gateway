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
	"context"
	"net"
	"net/http"
	"net/url"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/endpoints/openapi"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericfilters "k8s.io/apiserver/pkg/server/filters"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	basecompatibility "k8s.io/component-base/compatibility"
	baseversion "k8s.io/component-base/version"
	"k8s.io/klog/v2"
	netutils "k8s.io/utils/net"

	"github.com/oam-dev/cluster-gateway/pkg/config"
	"github.com/oam-dev/cluster-gateway/pkg/metrics"
	"github.com/oam-dev/cluster-gateway/pkg/options"
	"github.com/oam-dev/cluster-gateway/pkg/util/loopback"
	"github.com/oam-dev/cluster-gateway/pkg/util/scheme"
	"github.com/oam-dev/cluster-gateway/pkg/util/singleton"

	// +kubebuilder:scaffold:resource-imports
	clusterv1alpha1 "github.com/oam-dev/cluster-gateway/pkg/apis/cluster/v1alpha1"
	"github.com/oam-dev/cluster-gateway/pkg/apis/generated"

	_ "github.com/oam-dev/cluster-gateway/pkg/featuregates"
)

// standaloneDebugMode allows the apiserver to be run locally without
// authorization/admission for testing, mirroring what
// sigs.k8s.io/apiserver-runtime's WithLocalDebugExtension used to provide.
var standaloneDebugMode bool

// parameterScheme/parameterCodec decode subresource Connect options (e.g.
// ClusterGatewayProxyOptions) from a request's raw query values.
var (
	parameterScheme = runtime.NewScheme()
	parameterCodec  = runtime.NewParameterCodec(parameterScheme)
)

func init() {
	// generic apiserver machinery needs the "empty group" v1 meta types
	// (metav1.Status and friends) registered as unversioned so it can encode
	// errors and discovery responses.
	metav1.AddToGroupVersion(scheme.Scheme, schema.GroupVersion{Version: "v1"})
	scheme.Scheme.AddUnversionedTypes(schema.GroupVersion{Group: "", Version: "v1"},
		&metav1.Status{},
		&metav1.APIVersions{},
		&metav1.APIGroupList{},
		&metav1.APIGroup{},
		&metav1.APIResourceList{},
	)

	metav1.AddMetaToScheme(parameterScheme)
	parameterScheme.AddKnownTypes(clusterv1alpha1.SchemeGroupVersion, &clusterv1alpha1.ClusterGatewayProxyOptions{})
	utilruntime.Must(parameterScheme.AddConversionFunc(&url.Values{}, &clusterv1alpha1.ClusterGatewayProxyOptions{},
		func(src, dest interface{}, _ conversion.Scope) error {
			return dest.(*clusterv1alpha1.ClusterGatewayProxyOptions).ConvertFromUrlValues(src.(*url.Values))
		}))
}

func main() {

	// registering metrics
	metrics.Register()

	cmd := newCommand()
	if err := cmd.Execute(); err != nil {
		klog.Fatal(err)
	}
}

func newCommand() *cobra.Command {
	codecs := serializer.NewCodecFactory(scheme.Scheme)
	o := genericoptions.NewRecommendedOptions("", codecs.LegacyCodec(clusterv1alpha1.SchemeGroupVersion))

	// cluster-gateway's resources are synthesized live from Secrets/OCM
	// ManagedClusters rather than persisted, so no etcd storage is needed.
	// Clear this before AddFlags so the etcd flags are never registered:
	// otherwise they would be accepted and silently ignored.
	o.Etcd = nil

	cmd := &cobra.Command{
		Use:          "cluster-gateway",
		Short:        "Launch the cluster-gateway aggregated apiserver",
		SilenceUsage: true,
		RunE: func(c *cobra.Command, args []string) error {
			return runServer(c.Context(), o)
		},
	}
	cmd.SetContext(genericapiserver.SetupSignalContext())

	flags := cmd.Flags()
	o.AddFlags(flags)
	utilfeature.DefaultMutableFeatureGate.AddFlag(flags)
	flags.BoolVar(&standaloneDebugMode, "standalone-debug-mode", false,
		"Under the local-debug mode the apiserver will allow all access to its resources without "+
			"authorizing the requests, this flag is only intended for debugging in your workstation "+
			"and the apiserver will be crashing if its binding address is not 127.0.0.1.")

	config.AddLogFlags(flags)
	config.AddSecretFlags(flags)
	config.AddVirtualClusterFlags(flags)
	config.AddClusterProxyFlags(flags)
	config.AddProxyAuthorizationFlags(flags)
	config.AddUserAgentFlags(flags)
	config.AddClusterGatewayProxyConfig(flags)
	flags.BoolVarP(&options.OCMIntegration, "ocm-integration", "", false,
		"Enabling OCM integration, reading cluster CA and api endpoint from managed "+
			"cluster.")

	return cmd
}

func runServer(ctx context.Context, o *genericoptions.RecommendedOptions) error {
	if err := config.ValidateSecret(); err != nil {
		return err
	}
	if err := config.ValidateClusterProxy(); err != nil {
		return err
	}
	if err := clusterv1alpha1.LoadGlobalClusterGatewayProxyConfig(); err != nil {
		return err
	}

	if standaloneDebugMode {
		if o.SecureServing.BindAddress.String() != "127.0.0.1" {
			klog.Fatal(`--bind-address must be "127.0.0.1" if --standalone-debug-mode is set`)
		}
		// Keep delegated authentication, but make the remote kubeconfig
		// optional, as sigs.k8s.io/apiserver-runtime's WithLocalDebugExtension
		// did: requests without credentials then arrive as system:anonymous,
		// so filters that expect a user on the request context (max-in-flight,
		// APF) still work. Authorization and admission stay off for debugging,
		// while a normal run without delegated config still fails to start.
		o.Authentication.RemoteKubeConfigFileOptional = true
		o.Authorization = nil
		o.Admission = nil
	}

	if err := o.SecureServing.MaybeDefaultWithSelfSignedCerts(
		"localhost", nil, []net.IP{netutils.ParseIPSloppy("127.0.0.1")}); err != nil {
		return err
	}

	codecs := serializer.NewCodecFactory(scheme.Scheme)
	serverConfig := genericapiserver.NewRecommendedConfig(codecs)
	serverConfig.EffectiveVersion = basecompatibility.NewEffectiveVersionFromString(baseversion.DefaultKubeBinaryVersion, "", "")
	serverConfig.LongRunningFunc = func(r *http.Request, requestInfo *request.RequestInfo) bool {
		if requestInfo.Resource == config.MetaApiResourceName && requestInfo.Subresource == "proxy" {
			return true
		}
		return genericfilters.BasicLongRunningRequestCheck(sets.NewString("watch"), sets.NewString())(r, requestInfo)
	}

	// Must be set before o.ApplyTo: it wires authentication security
	// definitions into OpenAPIConfig only if already non-nil at that point.
	serverConfig.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(generated.GetOpenAPIDefinitions, openapi.NewDefinitionNamer(scheme.Scheme))
	serverConfig.OpenAPIConfig.Info.Title = "Cluster Gateway"
	serverConfig.OpenAPIConfig.Info.Version = "1.0.0"
	serverConfig.OpenAPIV3Config = genericapiserver.DefaultOpenAPIV3Config(generated.GetOpenAPIDefinitions, openapi.NewDefinitionNamer(scheme.Scheme))
	serverConfig.OpenAPIV3Config.Info.Title = "Cluster Gateway"
	serverConfig.OpenAPIV3Config.Info.Version = "1.0.0"

	if err := o.ApplyTo(serverConfig); err != nil {
		return err
	}

	config.WithUserAgent(serverConfig)

	genericServer, err := serverConfig.Complete().New("cluster-gateway", genericapiserver.NewEmptyDelegate())
	if err != nil {
		return err
	}
	genericServer.Handler.FullHandlerChain = clusterv1alpha1.NewClusterGatewayProxyRequestEscaper(genericServer.Handler.FullHandlerChain)

	clusterGatewayStorage := &clusterv1alpha1.ClusterGateway{}
	proxyStorage := &clusterv1alpha1.ClusterGatewayProxy{Parent: clusterGatewayStorage}
	healthStorage := &clusterv1alpha1.ClusterGatewayHealth{Parent: clusterGatewayStorage}

	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(config.MetaApiGroupName, scheme.Scheme, parameterCodec, codecs)
	apiGroupInfo.VersionedResourcesStorageMap[config.MetaApiVersionName] = map[string]rest.Storage{
		config.MetaApiResourceName:             clusterGatewayStorage,
		config.MetaApiResourceName + "/proxy":  proxyStorage,
		config.MetaApiResourceName + "/health": healthStorage,
		"virtualclusters":                      &clusterv1alpha1.VirtualCluster{},
	}
	if err := genericServer.InstallAPIGroup(&apiGroupInfo); err != nil {
		return err
	}

	loopback.SetLoopbackMasterClientConfig(serverConfig.ClientConfig)
	loopback.SetAuthorizer(genericServer.Authorizer)

	if err := genericServer.AddPostStartHook("init-master-loopback-client", singleton.InitLoopbackClient); err != nil {
		return err
	}

	return genericServer.PrepareRun().RunWithContext(ctx)
}
