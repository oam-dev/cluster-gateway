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

	"k8s.io/client-go/transport"

	"github.com/oam-dev/cluster-gateway/pkg/apis/cluster/v1alpha1"
	contextutil "github.com/oam-dev/cluster-gateway/pkg/util/context"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ClusterGatewayExpansion interface {
	RESTClient(clusterName string) rest.Interface

	GetKubernetesClient(clusterName string) kubernetes.Interface
	GetControllerRuntimeClient(clusterName string, options client.Options) (client.Client, error)

	RoundTripperForCluster(clusterName string) http.RoundTripper
	RoundTripperForClusterFromContext() http.RoundTripper
	RoundTripperForClusterFromContextWrapper(http.RoundTripper) http.RoundTripper
}

func (c *clusterGateways) RESTClient(clusterName string) rest.Interface {
	restClient := c.client.(*rest.RESTClient)
	shallowCopiedClient := *restClient
	shallowCopiedHTTPClient := *(restClient.Client)
	shallowCopiedClient.Client = &shallowCopiedHTTPClient
	shallowCopiedClient.Client.Transport = c.RoundTripperForCluster(clusterName)
	return &shallowCopiedClient
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
		Host:          c.client.Verb("").URL().String(),
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
	restClient := c.client.(*rest.RESTClient)
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
