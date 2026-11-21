package servingcert

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/oam-dev/cluster-gateway/pkg/config"
)

func newAggregatorScheme(t *testing.T) *runtime.Scheme {
	s := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(s))
	require.NoError(t, apiregistrationv1.AddToScheme(s))
	return s
}

func TestEnsureClusterGatewayTLS_Success(t *testing.T) {
	scheme := newAggregatorScheme(t)
	ctx := context.Background()

	svc := &Service{Namespace: "vela-system", Name: "cluster-gateway-service"}
	apiService := &apiregistrationv1.APIService{
		ObjectMeta: metav1.ObjectMeta{Name: DefaultAPIServiceName},
		Spec: apiregistrationv1.APIServiceSpec{
			Service: &apiregistrationv1.ServiceReference{
				Namespace: "vela-system",
				Name:      "cluster-gateway-service",
			},
			InsecureSkipTLSVerify: true,
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(apiService).Build()

	res, err := EnsureClusterGatewayTLS(ctx, c, "vela-system", "cluster-gateway-tls", svc, "")
	require.NoError(t, err)
	require.NotEmpty(t, res.CABundle)

	// Verify APIService has been patched
	updated := &apiregistrationv1.APIService{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: DefaultAPIServiceName}, updated))
	require.Equal(t, res.CABundle, updated.Spec.CABundle)
	require.False(t, updated.Spec.InsecureSkipTLSVerify)

	// Idempotent execution
	res2, err := EnsureClusterGatewayTLS(ctx, c, "vela-system", "cluster-gateway-tls", svc, "")
	require.NoError(t, err)
	require.Equal(t, res.CABundle, res2.CABundle)
}

func TestEnsureClusterGatewayTLS_PointsAtMismatch(t *testing.T) {
	scheme := newAggregatorScheme(t)
	ctx := context.Background()

	svc := &Service{Namespace: "vela-system", Name: "my-gateway-service"}
	// APIService points to a different service
	apiService := &apiregistrationv1.APIService{
		ObjectMeta: metav1.ObjectMeta{Name: DefaultAPIServiceName},
		Spec: apiregistrationv1.APIServiceSpec{
			Service: &apiregistrationv1.ServiceReference{
				Namespace: "other-ns",
				Name:      "other-service",
			},
			InsecureSkipTLSVerify: true,
			CABundle:              []byte("existing-ca"),
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(apiService).Build()

	res, err := EnsureClusterGatewayTLS(ctx, c, "vela-system", "cluster-gateway-tls", svc, "")
	require.NoError(t, err)
	require.NotEmpty(t, res.CABundle)

	// APIService must remain untouched!
	updated := &apiregistrationv1.APIService{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: DefaultAPIServiceName}, updated))
	require.Equal(t, []byte("existing-ca"), updated.Spec.CABundle)
	require.True(t, updated.Spec.InsecureSkipTLSVerify)
}

func TestEnsureClusterGatewayTLS_CertManagerAnnotation(t *testing.T) {
	scheme := newAggregatorScheme(t)
	ctx := context.Background()

	svc := &Service{Namespace: "vela-system", Name: "cluster-gateway-service"}
	apiService := &apiregistrationv1.APIService{
		ObjectMeta: metav1.ObjectMeta{
			Name: DefaultAPIServiceName,
			Annotations: map[string]string{
				CertManagerInjectCAAnnotation: "vela-system/cluster-gateway-cert",
			},
		},
		Spec: apiregistrationv1.APIServiceSpec{
			Service: &apiregistrationv1.ServiceReference{
				Namespace: "vela-system",
				Name:      "cluster-gateway-service",
			},
			InsecureSkipTLSVerify: true,
			CABundle:              []byte("cert-manager-ca"),
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(apiService).Build()

	_, err := EnsureClusterGatewayTLS(ctx, c, "vela-system", "cluster-gateway-tls", svc, "")
	require.NoError(t, err)

	// APIService caBundle must be untouched because cert-manager manages it
	updated := &apiregistrationv1.APIService{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: DefaultAPIServiceName}, updated))
	require.Equal(t, []byte("cert-manager-ca"), updated.Spec.CABundle)
	require.True(t, updated.Spec.InsecureSkipTLSVerify)
}

func TestEnsureClusterGatewayTLS_InferServiceFromAPIService(t *testing.T) {
	scheme := newAggregatorScheme(t)
	ctx := context.Background()

	apiService := &apiregistrationv1.APIService{
		ObjectMeta: metav1.ObjectMeta{Name: DefaultAPIServiceName},
		Spec: apiregistrationv1.APIServiceSpec{
			Service: &apiregistrationv1.ServiceReference{
				Namespace: "custom-ns",
				Name:      "inferred-service",
			},
			InsecureSkipTLSVerify: true,
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(apiService).Build()

	// Pass nil svc to infer from APIService
	res, err := EnsureClusterGatewayTLS(ctx, c, "custom-ns", "cluster-gateway-tls", nil, "")
	require.NoError(t, err)

	updated := &apiregistrationv1.APIService{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: DefaultAPIServiceName}, updated))
	require.Equal(t, res.CABundle, updated.Spec.CABundle)
	require.False(t, updated.Spec.InsecureSkipTLSVerify)
}

func TestEnsureClusterGatewayTLS_APIServiceNotFound(t *testing.T) {
	scheme := newAggregatorScheme(t)
	ctx := context.Background()

	svc := &Service{Namespace: "vela-system", Name: "cluster-gateway-service"}
	c := fake.NewClientBuilder().WithScheme(scheme).Build()

	// APIService does not exist in cluster yet; should not return error
	res, err := EnsureClusterGatewayTLS(ctx, c, "vela-system", "cluster-gateway-tls", svc, "")
	require.NoError(t, err)
	require.NotEmpty(t, res.Cert)
	require.NotEmpty(t, res.CABundle)
}

func TestBuildSchemeOrDie(t *testing.T) {
	s := buildSchemeOrDie()
	require.NotNil(t, s)
	require.True(t, s.Recognizes(apiregistrationv1.SchemeGroupVersion.WithKind("APIService")))
}

func TestRegisterPostStartHook_Nil(t *testing.T) {
	reconcilerHook = nil
	require.NoError(t, RegisterPostStartHook(nil))

	reconcilerHook = func(genericapiserver.PostStartHookContext) error { return nil }
	require.NoError(t, RegisterPostStartHook(nil))
	reconcilerHook = nil
}

func TestHasCertManagerAnnotation(t *testing.T) {
	apiSvcNil := &apiregistrationv1.APIService{}
	require.False(t, hasCertManagerAnnotation(apiSvcNil))

	apiSvcEmpty := &apiregistrationv1.APIService{
		ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{}},
	}
	require.False(t, hasCertManagerAnnotation(apiSvcEmpty))

	apiSvcCA := &apiregistrationv1.APIService{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{CertManagerInjectCAAnnotation: "ns/secret"},
		},
	}
	require.True(t, hasCertManagerAnnotation(apiSvcCA))

	apiSvcFromSecret := &apiregistrationv1.APIService{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{CertManagerInjectCAFromSecretAnnotation: "ns/secret"},
		},
	}
	require.True(t, hasCertManagerAnnotation(apiSvcFromSecret))

	apiSvcServerCA := &apiregistrationv1.APIService{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{CertManagerInjectAPIServerCAAnnotation: "true"},
		},
	}
	require.True(t, hasCertManagerAnnotation(apiSvcServerCA))
}

func TestPrepare_Options(t *testing.T) {
	ctx := context.Background()

	config.ClusterGatewayTLSSecret = ""
	require.NoError(t, Prepare(ctx, nil))

	config.ClusterGatewayTLSSecret = "too/many/slashes"
	require.Error(t, Prepare(ctx, nil))

	config.ClusterGatewayTLSSecret = "default/my-secret"
	config.ClusterGatewayService = "too/many/slashes"
	require.Error(t, Prepare(ctx, nil))

	// Conflicting flags
	opts := genericoptions.NewRecommendedOptions("", nil)
	opts.SecureServing.ServerCert.CertKey.CertFile = "/tmp/tls.crt"
	config.ClusterGatewayTLSSecret = "default/my-secret"
	config.ClusterGatewayService = "default/my-service"
	require.Error(t, Prepare(ctx, opts))

	// Conflicting flags with only KeyFile set
	optsOnlyKey := genericoptions.NewRecommendedOptions("", nil)
	optsOnlyKey.SecureServing.ServerCert.CertKey.KeyFile = "/tmp/tls.key"
	require.Error(t, Prepare(ctx, optsOnlyKey))

	// Reset config
	config.ClusterGatewayTLSSecret = ""
	config.ClusterGatewayService = ""
}

func TestEnsureClusterGatewayTLS_NoServiceRef(t *testing.T) {
	scheme := newAggregatorScheme(t)
	ctx := context.Background()

	// APIService with nil Service spec
	apiService := &apiregistrationv1.APIService{
		ObjectMeta: metav1.ObjectMeta{Name: DefaultAPIServiceName},
		Spec:       apiregistrationv1.APIServiceSpec{},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(apiService).Build()

	_, err := EnsureClusterGatewayTLS(ctx, c, "default", "my-secret", nil, "")
	require.Error(t, err)
}

func TestStartBackgroundReconciler(t *testing.T) {
	scheme := newAggregatorScheme(t)
	apiService := &apiregistrationv1.APIService{
		ObjectMeta: metav1.ObjectMeta{Name: DefaultAPIServiceName},
		Spec: apiregistrationv1.APIServiceSpec{
			Service: &apiregistrationv1.ServiceReference{
				Namespace: "default",
				Name:      "gateway-service",
			},
			InsecureSkipTLSVerify: true,
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(apiService).Build()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	certDir := t.TempDir()
	svc := &Service{Namespace: "default", Name: "gateway-service"}
	go StartBackgroundReconciler(ctx, c, "default", "gateway-tls", svc, DefaultAPIServiceName, certDir, 20*time.Millisecond)

	require.Eventually(t, func() bool {
		certFile := filepath.Join(certDir, CertKey)
		keyFile := filepath.Join(certDir, KeyKey)
		if _, err := os.Stat(certFile); err != nil {
			return false
		}
		if _, err := os.Stat(keyFile); err != nil {
			return false
		}
		updated := &apiregistrationv1.APIService{}
		if err := c.Get(ctx, types.NamespacedName{Name: DefaultAPIServiceName}, updated); err != nil {
			return false
		}
		return len(updated.Spec.CABundle) > 0 && !updated.Spec.InsecureSkipTLSVerify
	}, 5*time.Second, 50*time.Millisecond)
}
