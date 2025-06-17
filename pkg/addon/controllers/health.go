package controllers

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	multicluster "github.com/oam-dev/cluster-gateway/pkg/apis/cluster/transport"
	"github.com/oam-dev/cluster-gateway/pkg/apis/cluster/v1alpha1"
	"github.com/oam-dev/cluster-gateway/pkg/common"
	"github.com/oam-dev/cluster-gateway/pkg/event"
	"github.com/oam-dev/cluster-gateway/pkg/generated/clientset/versioned"
)

var (
	healthLog = ctrl.Log.WithName("ClusterGatewayHealthProber")
)
var _ reconcile.Reconciler = &ClusterGatewayHealthProber{}

type ClusterGatewayHealthProber struct {
	multiClusterRestClient rest.Interface
	gatewayClient          versioned.Interface
	runtimeClient          client.Client
}

func SetupClusterGatewayHealthProberWithManager(mgr ctrl.Manager) error {
	gatewayClient, err := versioned.NewForConfig(mgr.GetConfig())
	if err != nil {
		return err
	}
	copied := rest.CopyConfig(mgr.GetConfig())
	copied.WrapTransport = multicluster.NewClusterGatewayRoundTripper
	multiClusterClient, err := kubernetes.NewForConfig(copied)
	if err != nil {
		return err
	}
	prober := &ClusterGatewayHealthProber{
		multiClusterRestClient: multiClusterClient.Discovery().RESTClient(),
		gatewayClient:          gatewayClient,
		runtimeClient:          mgr.GetClient(),
	}
	src := event.AddOnHealthResyncHandler(mgr.GetClient(), time.Second)
	return ctrl.NewControllerManagedBy(mgr).
		For(&addonv1alpha1.ManagedClusterAddOn{}).
		WatchesRawSource(src).
		Complete(prober)
}

func (c *ClusterGatewayHealthProber) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	if request.Name != common.AddonName {
		return reconcile.Result{}, nil
	}
	clusterName := request.Namespace
	gw, err := c.gatewayClient.ClusterV1alpha1().
		ClusterGateways().
		GetHealthiness(ctx, clusterName, metav1.GetOptions{})
	if err != nil {
		return reconcile.Result{}, err
	}
	resp, healthErr := c.multiClusterRestClient.
		Get().
		AbsPath("healthz").
		DoRaw(multicluster.WithMultiClusterContext(context.TODO(), clusterName))
	healthy := string(resp) == "ok" && healthErr == nil
	if !healthy {
		healthErrMsg := ""
		if healthErr != nil {
			healthErrMsg = healthErr.Error()
		}
		healthLog.Info("Cluster unhealthy", "cluster", clusterName,
			"body", string(resp),
			"error", healthErrMsg)
	}
	if healthy != gw.Status.Healthy {
		gw.Status.Healthy = healthy
		if !healthy {
			if healthErr != nil {
				gw.Status.HealthyReason = v1alpha1.HealthyReasonType(healthErr.Error())
			}
		} else {
			gw.Status.HealthyReason = ""
		}
		healthLog.Info("Updating cluster healthiness",
			"cluster", clusterName,
			"healthy", healthy)
		_, err = c.gatewayClient.ClusterV1alpha1().
			ClusterGateways().
			UpdateHealthiness(ctx, gw, metav1.UpdateOptions{})
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	addon := &addonv1alpha1.ManagedClusterAddOn{}
	if err := c.runtimeClient.Get(ctx, request.NamespacedName, addon); err != nil {
		return reconcile.Result{}, err
	}
	if healthy != meta.IsStatusConditionTrue(addon.Status.Conditions, addonv1alpha1.ManagedClusterAddOnConditionAvailable) {
		healthLog.Info("Updating addon healthiness",
			"cluster", clusterName,
			"healthy", healthy)
		healthyStatus := metav1.ConditionTrue
		if !healthy {
			healthyStatus = metav1.ConditionFalse
		}
		if healthy {
			meta.SetStatusCondition(&addon.Status.Conditions, metav1.Condition{
				Type:    addonv1alpha1.ManagedClusterAddOnConditionAvailable,
				Status:  healthyStatus,
				Reason:  "SuccessfullyProbedHealthz",
				Message: "Returned OK",
			})
		} else {
			errMsg := "Unknown"
			if healthErr != nil {
				errMsg = healthErr.Error()
			} else if len(string(resp)) > 0 {
				errMsg = string(resp)
			}
			meta.SetStatusCondition(&addon.Status.Conditions, metav1.Condition{
				Type:    addonv1alpha1.ManagedClusterAddOnConditionAvailable,
				Status:  healthyStatus,
				Reason:  "FailedProbingHealthz",
				Message: errMsg,
			})
		}
		if err := c.runtimeClient.Status().Update(ctx, addon); err != nil {
			return reconcile.Result{}, err
		}
	}

	if !healthy {
		return reconcile.Result{
			Requeue:      true,
			RequeueAfter: 5 * time.Second,
		}, nil
	}
	return reconcile.Result{}, nil
}
