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

package v1alpha1_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	clusterv1alpha1 "github.com/oam-dev/cluster-gateway/pkg/apis/cluster/v1alpha1"
	"github.com/oam-dev/cluster-gateway/pkg/generated/clientset/versioned"
	"k8s.io/client-go/rest"
)

// recordingRoundTripper stands in for the transport of the original REST
// client so that the test can observe the requests a per-cluster client sends.
type recordingRoundTripper struct {
	paths []string
}

func (rt *recordingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.paths = append(rt.paths, req.URL.Path)
	return &http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(`{"kind":"ClusterGateway","apiVersion":"` +
			clusterv1alpha1.SchemeGroupVersion.String() + `","metadata":{"name":"member"}}`)),
		Request: req,
	}, nil
}

func (rt *recordingRoundTripper) lastPath() string {
	if len(rt.paths) == 0 {
		return ""
	}
	return rt.paths[len(rt.paths)-1]
}

func TestRESTClientForCluster(t *testing.T) {
	clientset, err := versioned.NewForConfig(&rest.Config{Host: "https://cluster-gateway.test"})
	if err != nil {
		t.Fatalf("failed to build clientset: %v", err)
	}

	original, ok := clientset.ClusterV1alpha1().RESTClient().(*rest.RESTClient)
	if !ok {
		t.Fatalf("unexpected REST client type %T", clientset.ClusterV1alpha1().RESTClient())
	}
	// replace the transport of the original client with an observable stub:
	// building the per-cluster copy must leave it in place
	recorder := &recordingRoundTripper{}
	original.Client.Transport = recorder

	gateways := clientset.ClusterV1alpha1().ClusterGateways()
	perCluster, ok := gateways.RESTClient("member").(*rest.RESTClient)
	if !ok {
		t.Fatalf("unexpected REST client type %T", gateways.RESTClient("member"))
	}

	if perCluster.Client == original.Client {
		t.Error(`RESTClient("member") shares its *http.Client with the original client`)
	}
	if perCluster.Client.Transport == recorder {
		t.Error(`RESTClient("member") does not use the round tripper of the target cluster`)
	}

	// the copy must resolve the same URLs as the original client, both for
	// regular requests (base path + versioned API path) and for AbsPath
	// (base path only)
	for _, tc := range []struct {
		name     string
		original string
		perClust string
	}{
		{"bare", original.Get().URL().String(), perCluster.Get().URL().String()},
		{
			"resource",
			original.Get().Resource("clustergateways").Name("member").URL().String(),
			perCluster.Get().Resource("clustergateways").Name("member").URL().String(),
		},
		{
			"abs path",
			original.Get().AbsPath("/api").URL().String(),
			perCluster.Get().AbsPath("/api").URL().String(),
		},
	} {
		if tc.perClust != tc.original {
			t.Errorf("%s: per-cluster request resolves to %q, want %q", tc.name, tc.perClust, tc.original)
		}
	}

	// ...but the request is sent to the target cluster through the proxy
	// subresource, and the response is decoded with the client's own codecs
	ctx := context.Background()
	if err := perCluster.Get().AbsPath("/api/v1/namespaces/default/pods").
		Do(ctx).Into(&clusterv1alpha1.ClusterGateway{}); err != nil {
		t.Fatalf("the per-cluster request failed: %v", err)
	}
	wantPath := "/apis/" + clusterv1alpha1.SchemeGroupVersion.Group +
		"/" + clusterv1alpha1.SchemeGroupVersion.Version +
		"/clustergateways/member/proxy/api/v1/namespaces/default/pods"
	if got := recorder.lastPath(); got != wantPath {
		t.Errorf("the per-cluster request was sent to %q, want %q", got, wantPath)
	}

	// the original client keeps talking to the aggregated apiserver itself
	if original.Client.Transport != recorder {
		t.Error("the transport of the original client was replaced")
	}
	if _, err := original.Get().AbsPath("/api/v1/namespaces").Do(ctx).Raw(); err != nil {
		t.Fatalf("the original request failed: %v", err)
	}
	if got, want := recorder.lastPath(), "/api/v1/namespaces"; got != want {
		t.Errorf("the original request was sent to %q, want %q", got, want)
	}
}

// TestRESTClientForClusterPreservesBasePath pins the base-path/versioned-API-path
// split that rebuilding the client depends on: a host carrying a path prefix has
// to keep resolving to the same URLs as the original client.
func TestRESTClientForClusterPreservesBasePath(t *testing.T) {
	for _, host := range []string{
		"https://cluster-gateway.test",
		"https://cluster-gateway.test/",
		"https://cluster-gateway.test/gateway",
		"https://cluster-gateway.test/gateway/",
	} {
		t.Run(host, func(t *testing.T) {
			clientset, err := versioned.NewForConfig(&rest.Config{Host: host})
			if err != nil {
				t.Fatalf("failed to build clientset: %v", err)
			}
			original := clientset.ClusterV1alpha1().RESTClient()
			perCluster := clientset.ClusterV1alpha1().ClusterGateways().RESTClient("member")
			if perCluster == nil {
				t.Fatal(`RESTClient("member") returned nil`)
			}

			for _, tc := range []struct {
				name     string
				original string
				perClust string
			}{
				{"bare", original.Get().URL().String(), perCluster.Get().URL().String()},
				{
					"resource",
					original.Get().Resource("clustergateways").Name("member").URL().String(),
					perCluster.Get().Resource("clustergateways").Name("member").URL().String(),
				},
				{
					"abs path",
					original.Get().AbsPath("/api").URL().String(),
					perCluster.Get().AbsPath("/api").URL().String(),
				},
			} {
				if tc.perClust != tc.original {
					t.Errorf("%s: per-cluster request resolves to %q, want %q", tc.name, tc.perClust, tc.original)
				}
			}
		})
	}
}
