package servingcert

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	errors2 "github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerruntimeconfig "sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/oam-dev/cluster-gateway/pkg/config"
)

const (
	// DefaultAPIServiceName is the default APIService name for cluster-gateway.
	DefaultAPIServiceName = "v1alpha1.cluster.core.oam.dev"

	// CertManagerInjectCAAnnotation is the annotation cert-manager cainjector looks for.
	CertManagerInjectCAAnnotation = "cert-manager.io/inject-ca-from"

	// CertManagerInjectCAFromSecretAnnotation is the annotation cert-manager cainjector looks for when injecting from a Secret.
	CertManagerInjectCAFromSecretAnnotation = "cert-manager.io/inject-ca-from-secret"

	// CertManagerInjectAPIServerCAAnnotation is another annotation cert-manager cainjector looks for.
	CertManagerInjectAPIServerCAAnnotation = "cert-manager.io/inject-apiserver-ca"
)

var reconcilerHook func(genericapiserver.PostStartHookContext) error

func buildSchemeOrDie() *runtime.Scheme {
	s := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(s))
	utilruntime.Must(apiregistrationv1.AddToScheme(s))
	return s
}

// Prepare configures serving certs and reconciles the APIService caBundle before
// the apiserver starts. If --cluster-gateway-tls-secret is not set, it is a no-op.
func Prepare(ctx context.Context, o *genericoptions.RecommendedOptions) error {
	if len(config.ClusterGatewayTLSSecret) == 0 {
		return nil
	}

	if o != nil && o.SecureServing != nil && (len(o.SecureServing.ServerCert.CertKey.CertFile) > 0 || len(o.SecureServing.ServerCert.CertKey.KeyFile) > 0) {
		return fmt.Errorf("cannot configure both --cluster-gateway-tls-secret and --tls-cert-file / --tls-private-key-file")
	}

	secretNamespace, secretName, err := config.ParseNamespacedName(config.ClusterGatewayTLSSecret, config.SecretNamespace)
	if err != nil {
		return err
	}

	var svc *Service
	if len(config.ClusterGatewayService) > 0 {
		svcNamespace, svcName, err := config.ParseNamespacedName(config.ClusterGatewayService, secretNamespace)
		if err != nil {
			return err
		}
		svc = &Service{Namespace: svcNamespace, Name: svcName}
	}

	certDir := config.ClusterGatewayCertDir
	if len(certDir) == 0 {
		certDir = filepath.Join(os.TempDir(), "cluster-gateway-certs")
	}

	restConfig, err := controllerruntimeconfig.GetConfig()
	if err != nil {
		return errors2.Wrapf(err, "failed to get kubeconfig for serving cert management")
	}
	ctrlClient, err := client.New(restConfig, client.Options{Scheme: buildSchemeOrDie()})
	if err != nil {
		return errors2.Wrapf(err, "failed to create client for serving cert management")
	}

	res, err := EnsureClusterGatewayTLS(ctx, ctrlClient, secretNamespace, secretName, svc, config.ClusterGatewayAPIService)
	if err != nil {
		return errors2.Wrapf(err, "failed to ensure cluster-gateway serving cert and caBundle")
	}

	if o != nil && o.SecureServing != nil && len(o.SecureServing.ServerCert.CertKey.CertFile) == 0 && res != nil {
		certFile := filepath.Join(certDir, CertKey)
		keyFile := filepath.Join(certDir, KeyKey)
		if _, err := WriteFileIfChanged(certFile, res.Cert); err != nil {
			return errors2.Wrapf(err, "failed to write serving cert file")
		}
		if _, err := WriteFileIfChanged(keyFile, res.Key); err != nil {
			return errors2.Wrapf(err, "failed to write serving key file")
		}
		o.SecureServing.ServerCert.CertKey.CertFile = certFile
		o.SecureServing.ServerCert.CertKey.KeyFile = keyFile
	}

	reconcilerHook = func(hookCtx genericapiserver.PostStartHookContext) error {
		go StartBackgroundReconciler(
			hookCtx,
			ctrlClient,
			secretNamespace,
			secretName,
			svc,
			config.ClusterGatewayAPIService,
			certDir,
			config.ClusterGatewayTLSResyncPeriod,
		)
		return nil
	}
	return nil
}

// RegisterPostStartHook registers the serving cert reconciler as a post-start hook
// on the generic apiserver if self-managed certificates are enabled.
func RegisterPostStartHook(s *genericapiserver.GenericAPIServer) error {
	if reconcilerHook == nil || s == nil {
		return nil
	}
	return s.AddPostStartHook("cluster-gateway-serving-cert-reconciler", reconcilerHook)
}

// EnsureClusterGatewayTLS makes sure that the cluster-gateway serving cert in
// the Secret <secretNamespace>/<secretName> is valid for the Service svc. It
// also reconciles the APIService caBundle, but only if the APIService points
// at svc and is not externally managed by cert-manager. If svc is nil, it deduces
// the Service from the APIService spec.
func EnsureClusterGatewayTLS(ctx context.Context, c client.Client, secretNamespace, secretName string, svc *Service, apiServiceName string) (*Result, error) {
	if apiServiceName == "" {
		apiServiceName = DefaultAPIServiceName
	}

	apiService := &apiregistrationv1.APIService{}
	var found bool

	if svc == nil {
		pollInterval := 100 * time.Millisecond
		pollTimeout := 10 * time.Second
		getErr := c.Get(ctx, types.NamespacedName{Name: apiServiceName}, apiService)
		if getErr != nil && apierrors.IsNotFound(getErr) {
			pollErr := wait.PollUntilContextTimeout(ctx, pollInterval, pollTimeout, true, func(stepCtx context.Context) (bool, error) {
				if err := c.Get(stepCtx, types.NamespacedName{Name: apiServiceName}, apiService); err == nil {
					return true, nil
				} else if !apierrors.IsNotFound(err) {
					return false, err
				}
				return false, nil
			})
			if pollErr != nil {
				return nil, errors2.Wrapf(pollErr, "failed waiting for APIService %s to deduce cluster-gateway Service", apiServiceName)
			}
		} else if getErr != nil {
			return nil, errors2.Wrapf(getErr, "failed to get APIService %s", apiServiceName)
		}
		found = true
		ref := apiService.Spec.Service
		if ref == nil {
			return nil, fmt.Errorf("APIService %s has no service reference, cannot determine TLS SANs", apiServiceName)
		}
		svc = &Service{Namespace: ref.Namespace, Name: ref.Name}
	} else {
		err := c.Get(ctx, types.NamespacedName{Name: apiServiceName}, apiService)
		if err != nil && !apierrors.IsNotFound(err) {
			return nil, errors2.Wrapf(err, "failed to get APIService %s", apiServiceName)
		}
		found = err == nil
	}

	res, err := EnsureSecret(ctx, c, secretNamespace, secretName, *svc)
	if err != nil {
		return nil, err
	}

	if !found {
		klog.V(2).InfoS("APIService not found yet; skipping caBundle reconcile for now", "apiService", apiServiceName)
		return res, nil
	}

	if !pointsAt(apiService, *svc) {
		klog.V(2).InfoS("Not setting the caBundle of the APIService: it does not point at cluster-gateway Service",
			"apiService", apiServiceName, "service", svc.Namespace+"/"+svc.Name)
		return res, nil
	}

	if hasCertManagerAnnotation(apiService) {
		klog.V(2).InfoS("Not setting the caBundle of the APIService: managed by cert-manager", "apiService", apiServiceName)
		return res, nil
	}

	if err = ensureAPIServiceCABundle(ctx, c, apiServiceName, res.CABundle, *svc); err != nil {
		return nil, err
	}
	return res, nil
}

// pointsAt reports whether the APIService forwards requests to the Service svc.
func pointsAt(apiService *apiregistrationv1.APIService, svc Service) bool {
	ref := apiService.Spec.Service
	return ref != nil && ref.Namespace == svc.Namespace && ref.Name == svc.Name
}

func hasCertManagerAnnotation(apiService *apiregistrationv1.APIService) bool {
	if apiService.Annotations == nil {
		return false
	}
	if apiService.Annotations[CertManagerInjectCAAnnotation] != "" ||
		apiService.Annotations[CertManagerInjectCAFromSecretAnnotation] != "" ||
		apiService.Annotations[CertManagerInjectAPIServerCAAnnotation] == "true" {
		return true
	}
	return false
}

// ensureAPIServiceCABundle sets the caBundle of the target APIService. Each attempt
// re-reads the APIService to ensure it still points to svc and is not externally managed.
func ensureAPIServiceCABundle(ctx context.Context, c client.Client, apiServiceName string, ca []byte, svc Service) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		apiService := &apiregistrationv1.APIService{}
		if err := c.Get(ctx, types.NamespacedName{Name: apiServiceName}, apiService); err != nil {
			return err
		}
		if !pointsAt(apiService, svc) {
			return nil
		}
		if hasCertManagerAnnotation(apiService) {
			return nil
		}
		if bytes.Equal(apiService.Spec.CABundle, ca) && !apiService.Spec.InsecureSkipTLSVerify {
			return nil
		}
		apiService.Spec.CABundle = ca
		apiService.Spec.InsecureSkipTLSVerify = false
		if err := c.Update(ctx, apiService); err != nil {
			return err
		}
		klog.InfoS("Set the caBundle of the APIService", "apiService", apiServiceName)
		return nil
	})
	return errors2.Wrapf(err, "failed to set caBundle of APIService %s", apiServiceName)
}

// StartBackgroundReconciler runs a continuous reconciliation loop at the given interval.
// It checks cert expiration, renews the secret when necessary, writes updated cert files to
// certDir (enabling dynamic cert reloading), and repairs the APIService caBundle if reset.
func StartBackgroundReconciler(ctx context.Context, c client.Client, secretNamespace, secretName string, svc *Service, apiServiceName, certDir string, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}
	if apiServiceName == "" {
		apiServiceName = DefaultAPIServiceName
	}
	klog.InfoS("Starting serving cert background reconciler", "secret", secretNamespace+"/"+secretName, "interval", interval.String())

	// Fast initial retries if APIService was missing or caBundle unset during boot
	go func() {
		_ = wait.PollUntilContextTimeout(ctx, 2*time.Second, interval, true, func(pollCtx context.Context) (bool, error) {
			apiService := &apiregistrationv1.APIService{}
			if err := c.Get(pollCtx, types.NamespacedName{Name: apiServiceName}, apiService); err != nil {
				return false, nil
			}
			if len(apiService.Spec.CABundle) > 0 && !apiService.Spec.InsecureSkipTLSVerify {
				return true, nil
			}
			res, err := EnsureClusterGatewayTLS(pollCtx, c, secretNamespace, secretName, svc, apiServiceName)
			if err == nil && certDir != "" && res != nil {
				certFile := filepath.Join(certDir, CertKey)
				keyFile := filepath.Join(certDir, KeyKey)
				_, _ = WriteFileIfChanged(certFile, res.Cert)
				_, _ = WriteFileIfChanged(keyFile, res.Key)
			}
			return len(apiService.Spec.CABundle) > 0 && !apiService.Spec.InsecureSkipTLSVerify, nil
		})
	}()

	wait.UntilWithContext(ctx, func(stepCtx context.Context) {
		res, err := EnsureClusterGatewayTLS(stepCtx, c, secretNamespace, secretName, svc, apiServiceName)
		if err != nil {
			klog.ErrorS(err, "Failed to reconcile cluster-gateway serving cert and caBundle")
			return
		}
		if certDir != "" && res != nil {
			certFile := filepath.Join(certDir, CertKey)
			keyFile := filepath.Join(certDir, KeyKey)
			if _, err := WriteFileIfChanged(certFile, res.Cert); err != nil {
				klog.ErrorS(err, "Failed to write renewed cert file", "path", certFile)
			}
			if _, err := WriteFileIfChanged(keyFile, res.Key); err != nil {
				klog.ErrorS(err, "Failed to write renewed key file", "path", keyFile)
			}
		}
	}, interval)
}
