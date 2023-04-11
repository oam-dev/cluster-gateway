/*


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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog"
	ocmclusterv1 "open-cluster-management.io/api/cluster/v1"

	"github.com/oam-dev/cluster-gateway/pkg/config"

	"github.com/oam-dev/cluster-gateway/pkg/util/scheme"
)

func init() {
	for _, fn := range []func(*runtime.Scheme) error{
		AddToScheme,
		corev1.AddToScheme,
		ocmclusterv1.Install,
	} {
		if err := fn(scheme.Scheme); err != nil {
			klog.Fatalf("failed registering core api types")
		}
	}
}

var AddToScheme = func(scheme *runtime.Scheme) error {
	metav1.AddToGroupVersion(scheme, schema.GroupVersion{
		Group:   config.MetaApiGroupName,
		Version: config.MetaApiVersionName,
	})
	// +kubebuilder:scaffold:install

	scheme.AddKnownTypes(schema.GroupVersion{
		Group:   config.MetaApiGroupName,
		Version: config.MetaApiVersionName,
	}, &ClusterGateway{}, &ClusterGatewayList{})
	scheme.AddKnownTypes(schema.GroupVersion{
		Group:   config.MetaApiGroupName,
		Version: config.MetaApiVersionName,
	}, &ClusterGatewayProxyOptions{})

	scheme.AddKnownTypes(schema.GroupVersion{
		Group:   config.MetaApiGroupName,
		Version: config.MetaApiVersionName,
	}, &VirtualCluster{}, &VirtualClusterList{})

	return nil
}

var SchemeGroupVersion = schema.GroupVersion{Group: config.MetaApiGroupName, Version: config.MetaApiVersionName}
