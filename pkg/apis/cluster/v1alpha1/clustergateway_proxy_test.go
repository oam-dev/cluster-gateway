package v1alpha1

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	gopath "path"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/util/feature"
	clientgorest "k8s.io/client-go/rest"
	"k8s.io/component-base/featuregate"
	k8stesting "k8s.io/component-base/featuregate/testing"
	"k8s.io/utils/pointer"
	contextutil "sigs.k8s.io/apiserver-runtime/pkg/util/context"
)

func TestProxyHandler(t *testing.T) {
	cases := []struct {
		name            string
		parent          *fakeParentStorage
		featureGate     featuregate.Feature
		objName         string
		inputOption     *ClusterGatewayProxyOptions
		reqInfo         request.RequestInfo
		query           string
		expectedQuery   string
		endpointPath    string
		expectedFailure bool
		errorAssertFunc func(t *testing.T, err error)
	}{
		{
			name: "normal proxy should work",
			parent: &fakeParentStorage{
				obj: &ClusterGateway{
					ObjectMeta: metav1.ObjectMeta{
						Name: "myName",
					},
					Spec: ClusterGatewaySpec{
						Access: ClusterAccess{
							Credential: &ClusterAccessCredential{
								Type:                CredentialTypeServiceAccountToken,
								ServiceAccountToken: "myToken",
							},
						},
					},
				},
			},
			objName: "myName",
			inputOption: &ClusterGatewayProxyOptions{
				Path: "/abc",
			},
			reqInfo: request.RequestInfo{
				Verb: "get",
			},
		},
		{
			name: "not found",
			parent: &fakeParentStorage{
				err: apierrors.NewNotFound(schema.GroupResource{}, ""),
			},
			objName: "myName",
			inputOption: &ClusterGatewayProxyOptions{
				Path: "/abc",
			},
			reqInfo: request.RequestInfo{
				Verb: "get",
			},
			expectedFailure: true,
			errorAssertFunc: func(t *testing.T, err error) {
				assert.True(t, strings.HasPrefix(err.Error(), "no such cluster"))
			},
		},
		{
			name: "normal proxy with sub-path in endpoint should work",
			parent: &fakeParentStorage{
				obj: &ClusterGateway{
					ObjectMeta: metav1.ObjectMeta{
						Name: "myName",
					},
					Spec: ClusterGatewaySpec{
						Access: ClusterAccess{
							Credential: &ClusterAccessCredential{
								Type:                CredentialTypeServiceAccountToken,
								ServiceAccountToken: "myToken",
							},
						},
					},
				},
			},
			endpointPath: "/extra",
			objName:      "myName",
			inputOption: &ClusterGatewayProxyOptions{
				Path: "/abc",
			},
			reqInfo: request.RequestInfo{
				Verb: "get",
			},
		},
		{
			name: "normal proxy with query in endpoint should work",
			parent: &fakeParentStorage{
				obj: &ClusterGateway{
					ObjectMeta: metav1.ObjectMeta{
						Name: "myName",
					},
					Spec: ClusterGatewaySpec{
						Access: ClusterAccess{
							Credential: &ClusterAccessCredential{
								Type:                CredentialTypeServiceAccountToken,
								ServiceAccountToken: "myToken",
							},
						},
					},
				},
			},
			objName: "myName",
			inputOption: &ClusterGatewayProxyOptions{
				Path: "/abc",
			},
			query:         "__dryRun=All&fieldValidation=Strict",
			expectedQuery: "dryRun=All&fieldValidation=Strict",
			reqInfo: request.RequestInfo{
				Verb: "get",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if len(c.featureGate) > 0 {
				k8stesting.SetFeatureGateDuringTest(t, feature.DefaultMutableFeatureGate, c.featureGate, true)
			}
			text := "ok"
			var receivingReq *http.Request
			endpointSvr := httptest.NewTLSServer(http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
				resp.WriteHeader(200)
				resp.Write([]byte(text))
				receivingReq = req
			}))
			defer endpointSvr.Close()
			if c.parent.obj != nil {
				c.parent.obj.Spec.Access.Endpoint = &ClusterEndpoint{
					Type: ClusterEndpointTypeConst,
					Const: &ClusterEndpointConst{
						Address:  endpointSvr.URL + c.endpointPath,
						Insecure: pointer.Bool(true),
					},
				}
			}

			ctx := context.TODO()
			ctx = contextutil.WithParentStorage(ctx, c.parent)
			ctx = request.WithRequestInfo(ctx, &c.reqInfo)

			gwProxy := &ClusterGatewayProxy{}
			handler, err := gwProxy.Connect(ctx, c.objName, c.inputOption, nil)
			if c.expectedFailure {
				if c.errorAssertFunc != nil {
					c.errorAssertFunc(t, err)
				}
				return
			}
			require.NoError(t, err)
			svr := httptest.NewServer(handler)
			defer svr.Close()
			path := "/foo"
			targetPath := apiPrefix + c.objName + apiSuffix + path
			resp, err := svr.Client().Get(svr.URL + targetPath + "?" + c.query)
			assert.NoError(t, err)
			data, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Equal(t, text, string(data))
			assert.Equal(t, 200, resp.StatusCode)
			assert.Equal(t, gopath.Join(c.endpointPath, path), receivingReq.URL.Path)
			assert.Equal(t, c.expectedQuery, receivingReq.URL.RawQuery)
		})
	}
}

var _ rest.Storage = &fakeParentStorage{}
var _ rest.Getter = &fakeParentStorage{}

type fakeParentStorage struct {
	obj *ClusterGateway
	err error
}

func (f *fakeParentStorage) New() runtime.Object {
	return f.obj
}

func (f *fakeParentStorage) Destroy() {}

func (f *fakeParentStorage) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	return f.obj, f.err
}

var _ rest.Responder = &fakeResponder{}

type fakeResponder struct {
	receivingCode int
	receivingObj  runtime.Object
	receivingErr  error
}

func (f *fakeResponder) Object(statusCode int, obj runtime.Object) {
	f.receivingCode = statusCode
	f.receivingObj = obj
}

func (f *fakeResponder) Error(err error) {
	f.receivingErr = err
}

func TestGetImpersonationConfig(t *testing.T) {
	baseReq, err := http.NewRequest(http.MethodGet, "", nil)
	require.NoError(t, err)
	base := context.Background()

	GlobalClusterGatewayProxyConfiguration = &ClusterGatewayProxyConfiguration{
		Spec: ClusterGatewayProxyConfigurationSpec{
			ClientIdentityExchanger: ClientIdentityExchanger{Rules: []ClientIdentityExchangeRule{{
				Name:   "name-matcher",
				Type:   StaticMappingIdentityExchanger,
				Source: &IdentityExchangerSource{User: pointer.String("test")},
				Target: &IdentityExchangerTarget{User: "global"},
			}}},
		},
	}

	h := &proxyHandler{clusterGateway: &ClusterGateway{Spec: ClusterGatewaySpec{ProxyConfig: &ClusterGatewayProxyConfiguration{
		Spec: ClusterGatewayProxyConfigurationSpec{
			ClientIdentityExchanger: ClientIdentityExchanger{Rules: []ClientIdentityExchangeRule{{
				Name:   "group-matcher",
				Type:   StaticMappingIdentityExchanger,
				Source: &IdentityExchangerSource{Group: pointer.String("group")},
				Target: &IdentityExchangerTarget{User: "local"},
			}}},
		},
	}}}}

	ctx := request.WithUser(base, &user.DefaultInfo{Name: "test", Groups: []string{"group"}})
	require.Equal(t, clientgorest.ImpersonationConfig{UserName: "local"}, h.getImpersonationConfig(baseReq.WithContext(ctx)))

	ctx = request.WithUser(base, &user.DefaultInfo{Name: "test", Groups: []string{"group-test"}})
	require.Equal(t, clientgorest.ImpersonationConfig{UserName: "global"}, h.getImpersonationConfig(baseReq.WithContext(ctx)))

	ctx = request.WithUser(base, &user.DefaultInfo{Name: "tester", Groups: []string{"group-test"}})
	require.Equal(t, clientgorest.ImpersonationConfig{UserName: "tester", Groups: []string{"group-test"}}, h.getImpersonationConfig(baseReq.WithContext(ctx)))
}
