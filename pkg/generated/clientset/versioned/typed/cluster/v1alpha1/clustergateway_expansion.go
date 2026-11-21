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

package v1alpha1

import (
	"context"
	"net/http"
	"strings"

	"github.com/oam-dev/cluster-gateway/pkg/apis/cluster/v1alpha1"
	"github.com/oam-dev/cluster-gateway/pkg/generated/clientset/versioned/scheme"
	contextutil "github.com/oam-dev/cluster-gateway/pkg/util/context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ClusterGatewayExpansion interface {
	RESTClient(clusterName string) rest.Interface

	GetKubernetesClient(clusterName string) kubernetes.Interface
	GetControllerRuntimeClient(clusterName string, options client.Options) (client.Client, error)

	RoundTripperForCluster(clusterName string) http.RoundTripper
	RoundTripperForClusterFromContext() http.RoundTripper
	RoundTripperForClusterFromContextWrapper(http.RoundTripper) http.RoundTripper

	GetHealthiness(ctx context.Context, name string, options metav1.GetOptions) (*v1alpha1.ClusterGateway, error)
	UpdateHealthiness(ctx context.Context, clusterGateway *v1alpha1.ClusterGateway, options metav1.UpdateOptions) (*v1alpha1.ClusterGateway, error)
}

func (c *clusterGateways) RESTClient(clusterName string) rest.Interface {
	restClient := c.GetClient().(*rest.RESTClient)

	// Rebuild the client with the public constructor instead of copying the
	// struct: rest.RESTClient keeps mutable private state (an atomic.Bool
	// tracking the CBOR fallback) that a shallow copy would read without
	// atomic operations, and whose layout may change in any client-go bump.
	base := restClient.Get().URL()
	// Request.URL() already joins the client's base path with its versioned
	// API path, so split the two again: the rebuilt client must resolve both
	// regular requests (base path + versioned API path) and AbsPath (base path
	// only) exactly like the original.
	basePath := restClient.Get().AbsPath().URL().Path
	versionedAPIPath := strings.TrimPrefix(base.Path, basePath)
	base.Path = basePath

	httpClient := *restClient.Client
	httpClient.Transport = c.RoundTripperForCluster(clusterName)

	groupVersion := restClient.APIVersion()
	newClient, err := rest.NewRESTClient(base, versionedAPIPath, rest.ClientContentConfig{
		GroupVersion: groupVersion,
		// the codec setup mirrors the generated setConfigDefaults
		Negotiator: runtime.NewClientNegotiator(
			rest.CodecFactoryForGeneratedClient(scheme.Scheme, scheme.Codecs).WithoutConversion(),
			groupVersion),
	}, restClient.GetRateLimiter(), &httpClient)
	if err != nil {
		// unreachable: every input is taken from an already working client
		klog.ErrorS(err, "failed to build the REST client for cluster", "cluster", clusterName)
		return nil
	}
	return newClient
}

func (c *clusterGateways) RoundTripperForCluster(clusterName string) http.RoundTripper {
	return c.getRoundTripper(func(ctx context.Context) string {
		return clusterName
	})
}

func (c *clusterGateways) GetKubernetesClient(clusterName string) kubernetes.Interface {
	return kubernetes.New(c.RESTClient(clusterName))
}

func (c *clusterGateways) GetControllerRuntimeClient(clusterName string, options client.Options) (client.Client, error) {
	return client.New(&rest.Config{
		Host:          c.GetClient().Verb("").URL().String(),
		WrapTransport: c.RoundTripperForClusterWrapperGenerator(clusterName),
	}, options)
}

func (c *clusterGateways) RoundTripperForClusterFromContext() http.RoundTripper {
	return c.getRoundTripper(contextutil.GetClusterName)
}

func (c *clusterGateways) RoundTripperForClusterFromContextWrapper(http.RoundTripper) http.RoundTripper {
	return c.RoundTripperForClusterFromContext()
}

func (c *clusterGateways) RoundTripperForClusterWrapperGenerator(clusterName string) transport.WrapperFunc {
	return func(rt http.RoundTripper) http.RoundTripper {
		return c.getRoundTripper(func(_ context.Context) string { return clusterName })
	}
}

func (c *clusterGateways) getRoundTripper(clusterNameGetter func(ctx context.Context) string) http.RoundTripper {
	restClient := c.GetClient().(*rest.RESTClient)
	return gatewayAPIPrefixPrepender{
		clusterNameGetter: clusterNameGetter,
		delegate:          restClient.Client.Transport,
	}
}

var _ http.RoundTripper = &gatewayAPIPrefixPrepender{}

type gatewayAPIPrefixPrepender struct {
	clusterNameGetter func(ctx context.Context) string
	delegate          http.RoundTripper
}

func (p gatewayAPIPrefixPrepender) RoundTrip(req *http.Request) (*http.Response, error) {
	originalPath := req.URL.Path
	prefix := "/apis/" +
		v1alpha1.SchemeGroupVersion.Group +
		"/" +
		v1alpha1.SchemeGroupVersion.Version +
		"/clustergateways/"
	if !strings.HasPrefix(originalPath, "/") {
		originalPath = "/" + originalPath
	}
	fullPath := prefix + p.clusterNameGetter(req.Context()) + "/proxy" + originalPath
	req.URL.Path = fullPath
	return p.delegate.RoundTrip(req)
}

func (c *clusterGateways) GetHealthiness(ctx context.Context, name string, options metav1.GetOptions) (*v1alpha1.ClusterGateway, error) {
	result := &v1alpha1.ClusterGateway{}
	err := c.GetClient().Get().
		Resource("clustergateways").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		SubResource("health").
		Do(ctx).
		Into(result)
	return result, err
}

func (c *clusterGateways) UpdateHealthiness(ctx context.Context, clusterGateway *v1alpha1.ClusterGateway, options metav1.UpdateOptions) (*v1alpha1.ClusterGateway, error) {
	result := &v1alpha1.ClusterGateway{}
	err := c.GetClient().Put().
		Resource("clustergateways").
		Name(clusterGateway.Name).
		VersionedParams(&options, scheme.ParameterCodec).
		Body(clusterGateway).
		SubResource("health").
		Do(ctx).
		Into(result)
	return result, err
}
