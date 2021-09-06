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
	"net/http"

	"github.com/oam-dev/cluster-gateway/pkg/apis/cluster/v1alpha1"
	"k8s.io/client-go/rest"
)

type ClusterGatewayExpansion interface {
	ForCluster(clusterName string) rest.Interface
}

func (c *clusterGateways) ForCluster(clusterName string) rest.Interface {
	restClient := c.client.(*rest.RESTClient)
	shallowCopiedClient := *restClient
	shallowCopiedHTTPClient := *(restClient.Client)
	shallowCopiedClient.Client = &shallowCopiedHTTPClient
	shallowCopiedClient.Client.Transport = gatewayAPIPrefixPrepender{
		clusterName: clusterName,
		delegate:    restClient.Client.Transport,
	}
	return &shallowCopiedClient
}

var _ http.RoundTripper = &gatewayAPIPrefixPrepender{}

type gatewayAPIPrefixPrepender struct {
	clusterName string
	delegate    http.RoundTripper
}

func (p gatewayAPIPrefixPrepender) RoundTrip(req *http.Request) (*http.Response, error) {
	originalPath := req.URL.Path
	prefix := "/apis/" +
		v1alpha1.SchemeGroupVersion.Group +
		"/" +
		v1alpha1.SchemeGroupVersion.Version +
		"/clustergateways/"
	fullPath := prefix + p.clusterName + "/proxy/" + originalPath
	req.URL.Path = fullPath
	return p.delegate.RoundTrip(req)
}
