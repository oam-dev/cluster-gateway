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

package fake

import (
	"net/http"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (c *FakeClusterGateways) ForCluster(clusterName string) rest.Interface {
	panic("not implemented")
}

func (c *FakeClusterGateways) RoundTripperForCluster(clusterName string) http.RoundTripper {
	panic("implement me")
}

func (c *FakeClusterGateways) RoundTripperForClusterFromContext() http.RoundTripper {
	panic("implement me")
}

func (c *FakeClusterGateways) RoundTripperForClusterFromContextWrapper(tripper http.RoundTripper) http.RoundTripper {
	panic("implement me")
}

func (c *FakeClusterGateways) RESTClient(clusterName string) rest.Interface {
	panic("implement me")
}

func (c *FakeClusterGateways) GetKubernetesClient(clusterName string) kubernetes.Interface {
	panic("implement me")
}

func (c *FakeClusterGateways) GetControllerRuntimeClient(clusterName string, options client.Options) (client.Client, error) {
	panic("implement me")
}
