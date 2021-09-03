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
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/oam-dev/cluster-gateway/pkg/metrics"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	restclient "k8s.io/client-go/rest"
	"sigs.k8s.io/apiserver-runtime/pkg/builder/resource"
	"sigs.k8s.io/apiserver-runtime/pkg/builder/resource/resourcerest"
	contextutil "sigs.k8s.io/apiserver-runtime/pkg/util/context"
)

var _ resource.SubResource = &ClusterGatewayProxy{}
var _ rest.Storage = &ClusterGatewayProxy{}
var _ resourcerest.Connecter = &ClusterGatewayProxy{}

var proxyMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}

// ClusterGatewayProxy is a subresource for ClusterGateway which allows user to proxy
// kubernetes resource requests to the managed cluster.
type ClusterGatewayProxy struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ClusterGatewayProxyOptions struct {
	metav1.TypeMeta

	// Path is the target api path of the proxy request.
	// e.g. "/healthz", "/api/v1"
	Path string `json:"path"`
}

func (c *ClusterGatewayProxy) SubResourceName() string {
	return "proxy"
}

func (c *ClusterGatewayProxy) New() runtime.Object {
	return &ClusterGatewayProxyOptions{}
}

func (c *ClusterGatewayProxy) Connect(ctx context.Context, id string, options runtime.Object, r rest.Responder) (http.Handler, error) {
	proxyOpts, ok := options.(*ClusterGatewayProxyOptions)
	if !ok {
		return nil, fmt.Errorf("invalid options object: %#v", options)
	}

	parentStorage, ok := contextutil.GetParentStorage(ctx)
	if !ok {
		return nil, fmt.Errorf("no parent storage found")
	}
	parentObj, err := parentStorage.Get(ctx, id, &metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("no such cluster %v", id)
	}

	reqInfo, _ := request.RequestInfoFrom(ctx)
	factory := request.RequestInfoFactory{
		APIPrefixes:          sets.NewString("api", "apis"),
		GrouplessAPIPrefixes: sets.NewString("api"),
	}
	proxyReqInfo, _ := factory.NewRequestInfo(&http.Request{
		URL: &url.URL{
			Path: proxyOpts.Path,
		},
		Method: strings.ToUpper(reqInfo.Verb),
	})
	proxyReqInfo.Verb = reqInfo.Verb

	return &proxyHandler{
		parentName:     id,
		path:           proxyOpts.Path,
		clusterGateway: parentObj.(*ClusterGateway),
		responder:      r,
		finishFunc: func(code int) {
			metrics.RecordProxiedRequestsByResource(proxyReqInfo.Resource, proxyReqInfo.Verb, code)
			metrics.RecordProxiedRequestsByCluster(id, code)
		},
	}, nil
}

func (c *ClusterGatewayProxy) NewConnectOptions() (runtime.Object, bool, string) {
	return &ClusterGatewayProxyOptions{}, true, "path"
}

func (c *ClusterGatewayProxy) ConnectMethods() []string {
	return proxyMethods
}

var _ resource.QueryParameterObject = &ClusterGatewayProxyOptions{}

func (in *ClusterGatewayProxyOptions) ConvertFromUrlValues(values *url.Values) error {
	in.Path = values.Get("path")
	return nil
}

var _ http.Handler = &proxyHandler{}

type proxyHandler struct {
	parentName     string
	path           string
	clusterGateway *ClusterGateway
	responder      rest.Responder
	finishFunc     func(code int)
}

func (p *proxyHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	cluster := p.clusterGateway
	if cluster.Spec.Access.Credential == nil {
		responsewriters.InternalError(writer, request, fmt.Errorf("proxying cluster %s not support due to lacking credentials", cluster.Name))
		return
	}

	// WithContext creates a shallow clone of the request with the same context.
	newReq := request.WithContext(request.Context())
	newReq.Header = utilnet.CloneHeader(request.Header)
	newReq.URL.Path = p.path

	urlAddr, err := GetEndpointURL(cluster)
	if err != nil {
		responsewriters.InternalError(writer, request, errors.Wrapf(err, "failed parsing endpoint for cluster %s", cluster.Name))
		return
	}
	host, _, _ := net.SplitHostPort(urlAddr.Host)
	newReq.Host = host
	newReq.Header.Add("Host", host)

	proxy := httputil.NewSingleHostReverseProxy(&url.URL{Scheme: urlAddr.Scheme, Host: urlAddr.Host})
	cfg, err := NewConfigFromCluster(cluster)
	if err != nil {
		responsewriters.InternalError(writer, request, errors.Wrapf(err, "failed creating cluster proxy client config %s", cluster.Name))
		return
	}
	rt, err := restclient.TransportFor(cfg)
	if err != nil {
		responsewriters.InternalError(writer, request, errors.Wrapf(err, "failed creating cluster proxy client %s", cluster.Name))
		return
	}

	const defaultFlushInterval = 200 * time.Millisecond
	proxy.Transport = rt
	proxy.FlushInterval = defaultFlushInterval
	proxy.ErrorLog = log.New(noSuppressPanicError{}, "", log.LstdFlags)
	if p.responder != nil {
		// if an optional error interceptor/responder was provided wire it
		// the custom responder might be used for providing a unified error reporting
		// or supporting retry mechanisms by not sending non-fatal errors to the clients
		proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
			p.responder.Error(err)
		}
	}
	proxy.ModifyResponse = func(r *http.Response) error {
		p.finishFunc(r.StatusCode)
		return nil
	}
	proxy.ServeHTTP(writer, newReq)
}

type noSuppressPanicError struct{}

func (noSuppressPanicError) Write(p []byte) (n int, err error) {
	// skip "suppressing panic for copyResponse error in test; copy error" error message
	// that ends up in CI tests on each kube-apiserver termination as noise and
	// everybody thinks this is fatal.
	if strings.Contains(string(p), "suppressing panic") {
		return len(p), nil
	}
	return os.Stderr.Write(p)
}
