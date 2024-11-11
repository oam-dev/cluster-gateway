package main

import (
	"context"
	"flag"
	"os"

	"github.com/oam-dev/cluster-gateway/pkg/addon/agent"
	"github.com/oam-dev/cluster-gateway/pkg/addon/controllers"
	proxyv1alpha1 "github.com/oam-dev/cluster-gateway/pkg/apis/proxy/v1alpha1"
	"github.com/oam-dev/cluster-gateway/pkg/util"
	"github.com/oam-dev/cluster-gateway/pkg/util/cert"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	nativescheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"open-cluster-management.io/addon-framework/pkg/addonmanager"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	ocmauthv1alpha1 "open-cluster-management.io/managed-serviceaccount/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = addonv1alpha1.AddToScheme(scheme)
	_ = proxyv1alpha1.AddToScheme(scheme)
	_ = nativescheme.AddToScheme(scheme)
	_ = apiregistrationv1.AddToScheme(scheme)
	_ = ocmauthv1alpha1.AddToScheme(scheme)
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var signerSecretName string

	logger := klogr.New()
	klog.SetOutput(os.Stdout)
	klog.InitFlags(flag.CommandLine)
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":48080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":48081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&signerSecretName, "signer-secret-name", "cluster-gateway-signer",
		"The name of the secret to store the signer CA")

	flag.Parse()
	ctrl.SetLogger(logger)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		WebhookServer: webhook.NewServer(webhook.Options{
			Port: 9443,
		}),
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "cluster-gateway-addon-manager",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	currentNamespace := os.Getenv("NAMESPACE")
	if len(currentNamespace) == 0 {
		inClusterNamespace, err := util.GetInClusterNamespace()
		if err != nil {
			klog.Fatal("the manager should be either running in a container or specify NAMESPACE environment")
		}
		currentNamespace = inClusterNamespace
	}

	caPair, err := cert.EnsureCAPair(mgr.GetConfig(), currentNamespace, signerSecretName)
	if err != nil {
		setupLog.Error(err, "unable to ensure ca signer")
	}
	nativeClient, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		setupLog.Error(err, "unable to instantiate legacy client")
		os.Exit(1)
	}
	informerFactory := informers.NewSharedInformerFactory(nativeClient, 0)
	if err := controllers.SetupClusterGatewayInstallerWithManager(
		mgr,
		caPair,
		nativeClient,
		informerFactory.Core().V1().Secrets().Lister()); err != nil {
		setupLog.Error(err, "unable to setup installer")
		os.Exit(1)
	}
	if err := controllers.SetupClusterGatewayHealthProberWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to setup health prober")
		os.Exit(1)
	}

	ctx := context.Background()
	go informerFactory.Start(ctx.Done())

	addonManager, err := addonmanager.New(mgr.GetConfig())
	if err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}
	if err := addonManager.AddAgent(agent.NewClusterGatewayAddonManager(
		mgr.GetConfig(),
		mgr.GetClient(),
	)); err != nil {
		setupLog.Error(err, "unable to register addon manager")
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(ctrl.SetupSignalHandler())
	defer cancel()
	go addonManager.Start(ctx)

	if err := mgr.Start(ctx); err != nil {
		panic(err)
	}

}
