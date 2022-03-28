package controllers

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	corev1lister "k8s.io/client-go/listers/core/v1"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"open-cluster-management.io/addon-framework/pkg/certrotation"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	ocmauthv1alpha1 "open-cluster-management.io/managed-serviceaccount/api/v1alpha1"

	clusterv1alpha1 "github.com/oam-dev/cluster-gateway/pkg/apis/cluster/v1alpha1"
	proxyv1alpha1 "github.com/oam-dev/cluster-gateway/pkg/apis/proxy/v1alpha1"
	"github.com/oam-dev/cluster-gateway/pkg/common"
	"github.com/oam-dev/cluster-gateway/pkg/event"
	"github.com/oam-dev/cluster-gateway/pkg/util/cert"
)

var (
	log = ctrl.Log.WithName("ClusterGatewayInstaller")
)
var _ reconcile.Reconciler = &ClusterGatewayInstaller{}

func SetupClusterGatewayInstallerWithManager(mgr ctrl.Manager, caPair *crypto.CA, nativeClient kubernetes.Interface, secretLister corev1lister.SecretLister) error {
	installer := &ClusterGatewayInstaller{
		nativeClient: nativeClient,
		caPair:       caPair,
		secretLister: secretLister,
		cache:        mgr.GetCache(),
		client:       mgr.GetClient(),
		mapper:       mgr.GetRESTMapper(),
	}
	return ctrl.NewControllerManagedBy(mgr).
		// Watches ClusterManagementAddOn singleton
		For(&addonv1alpha1.ClusterManagementAddOn{}).
		// Watches ClusterGatewayConfiguration singleton
		Watches(
			&source.Kind{
				Type: &proxyv1alpha1.ClusterGatewayConfiguration{},
			},
			&event.ClusterGatewayConfigurationHandler{
				Client: mgr.GetClient(),
			}).
		// Watches ManagedClusterAddon.
		Watches(
			&source.Kind{
				Type: &addonv1alpha1.ManagedClusterAddOn{},
			},
			&handler.EnqueueRequestForOwner{
				OwnerType: &addonv1alpha1.ClusterManagementAddOn{},
			}).
		// Cluster-Gateway mTLS certificate should be actively reconciled
		Watches(
			&source.Kind{
				Type: &corev1.Secret{},
			},
			&handler.EnqueueRequestForOwner{
				OwnerType: &addonv1alpha1.ClusterManagementAddOn{},
			}).
		// Secrets rotated by ManagedServiceAccount should be actively reconciled
		Watches(
			&source.Kind{
				Type: &corev1.Secret{},
			},
			&event.SecretHandler{}).
		// Cluster-gateway apiserver instances should be actively reconciled
		Watches(
			&source.Kind{
				Type: &appsv1.Deployment{},
			},
			&handler.EnqueueRequestForOwner{
				OwnerType: &addonv1alpha1.ClusterManagementAddOn{},
			}).
		// APIService should be actively reconciled
		Watches(
			&source.Kind{
				Type: &apiregistrationv1.APIService{},
			},
			&event.APIServiceHandler{WatchingName: common.ClusterGatewayAPIServiceName}).
		Complete(installer)
}

type ClusterGatewayInstaller struct {
	nativeClient kubernetes.Interface
	secretLister corev1lister.SecretLister
	caPair       *crypto.CA
	client       client.Client
	cache        cache.Cache
	mapper       meta.RESTMapper
}

const (
	SecretNameClusterGatewayTLSCert = "cluster-gateway-tls-cert"
	ServiceNameClusterGateway       = "cluster-gateway"
)

func (c *ClusterGatewayInstaller) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	// get the cluster-management-addon instance
	log.Info("Start reconciling")
	addon := &addonv1alpha1.ClusterManagementAddOn{}
	if err := c.client.Get(ctx, request.NamespacedName, addon); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, errors.Wrapf(err, "failed to get cluster-management-addon: %v", request.Name)
	}
	if addon.Name != common.AddonName {
		// skip
		return reconcile.Result{}, nil
	}

	if addon.Spec.AddOnConfiguration.CRDName != common.ClusterGatewayConfigurationCRDName {
		// skip
		return reconcile.Result{}, nil
	}

	clusterGatewayConfiguration := &proxyv1alpha1.ClusterGatewayConfiguration{}
	if err := c.client.Get(ctx, types.NamespacedName{
		Name: addon.Spec.AddOnConfiguration.CRName,
	}, clusterGatewayConfiguration); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("no such configuration: %v", addon.Spec.AddOnConfiguration.CRName)
		}
		return reconcile.Result{}, fmt.Errorf("failed getting configuration: %v", addon.Spec.AddOnConfiguration.CRName)
	}

	if err := c.ensureNamespace(clusterGatewayConfiguration.Spec.InstallNamespace); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to ensure required namespace")
	}
	if err := c.ensureNamespace(clusterGatewayConfiguration.Spec.SecretNamespace); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to ensure required namespace")
	}
	if err := c.ensureClusterProxySecrets(clusterGatewayConfiguration); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to ensure required proxy client related credentials")
	}
	if err := c.ensureSecretManagement(addon, clusterGatewayConfiguration); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to configure secret management")
	}

	sans := []string{
		ServiceNameClusterGateway,
		ServiceNameClusterGateway + "." + clusterGatewayConfiguration.Spec.InstallNamespace,
		ServiceNameClusterGateway + "." + clusterGatewayConfiguration.Spec.InstallNamespace + ".svc",
	}
	rotation := certrotation.TargetRotation{
		Namespace:     clusterGatewayConfiguration.Spec.InstallNamespace,
		Name:          SecretNameClusterGatewayTLSCert,
		HostNames:     sans,
		Validity:      time.Hour * 24 * 180,
		Lister:        c.secretLister,
		Client:        c.nativeClient.CoreV1(),
		EventRecorder: events.NewInMemoryRecorder("ClusterGatewayInstaller"),
	}
	if err := rotation.EnsureTargetCertKeyPair(c.caPair, c.caPair.Config.Certs); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed rotating server tls cert")
	}

	caCertData, _, err := c.caPair.Config.GetPEMBytes()
	if err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed encoding CA cert")
	}

	// create if not exists
	namespace := clusterGatewayConfiguration.Spec.InstallNamespace
	targets := []client.Object{
		newServiceAccount(addon, namespace),
		newClusterGatewayService(addon, namespace),
		newAuthenticationRole(addon, namespace),
		newSecretRole(addon, clusterGatewayConfiguration.Spec.SecretNamespace),
		newSecretRoleBinding(addon, namespace, clusterGatewayConfiguration.Spec.SecretNamespace),
		newAPFClusterRole(addon),
		newAPFClusterRoleBinding(addon, namespace),
		newAPIService(addon, namespace, caCertData),
	}
	for _, obj := range targets {
		if err := c.client.Create(context.TODO(), obj); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return reconcile.Result{}, errors.Wrapf(err, "failed deploying cluster-gateway")
			}
		}
	}

	if err := c.ensureClusterGatewayDeployment(addon, clusterGatewayConfiguration); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed ensuring cluster-gateway deployment")
	}

	// always update apiservice
	if err := c.ensureAPIService(addon, namespace); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed ensuring cluster-gateway apiservice")
	}

	return reconcile.Result{}, nil
}

func (c *ClusterGatewayInstaller) ensureNamespace(namespace string) error {
	ns := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	if _, err := c.nativeClient.CoreV1().Namespaces().Create(context.TODO(), ns, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}

func (c *ClusterGatewayInstaller) ensureAPIService(addon *addonv1alpha1.ClusterManagementAddOn, namespace string) error {
	caCertData, _, err := c.caPair.Config.GetPEMBytes()
	if err != nil {
		return err
	}
	expected := newAPIService(addon, namespace, caCertData)
	current := &apiregistrationv1.APIService{}
	if err := c.client.Get(context.TODO(), types.NamespacedName{
		Name: expected.Name,
	}, current); err != nil {
		return err
	}
	if !bytes.Equal(caCertData, current.Spec.CABundle) {
		expected.ResourceVersion = current.ResourceVersion
		if err := c.client.Update(context.TODO(), expected); err != nil {
			return err
		}
	}
	return nil
}

func (c *ClusterGatewayInstaller) ensureClusterGatewayDeployment(addon *addonv1alpha1.ClusterManagementAddOn, config *proxyv1alpha1.ClusterGatewayConfiguration) error {
	currentClusterGateway := &appsv1.Deployment{}
	if err := c.client.Get(context.TODO(), types.NamespacedName{
		Namespace: config.Spec.InstallNamespace,
		Name:      "gateway-deployment",
	}, currentClusterGateway); err != nil {
		if apierrors.IsNotFound(err) {
			clusterGateway := newClusterGatewayDeployment(addon, config)
			if err := c.client.Create(context.TODO(), clusterGateway); err != nil {
				return err
			}
			return nil
		}
		return err
	}
	genStr, ok := currentClusterGateway.Labels[labelKeyClusterGatewayConfigurationGeneration]
	if ok {
		gen, err := strconv.Atoi(genStr)
		if err != nil {
			return err
		}
		if config.Generation == int64(gen) {
			return nil
		}
	}

	clusterGateway := newClusterGatewayDeployment(addon, config)
	clusterGateway.ResourceVersion = currentClusterGateway.ResourceVersion
	if err := c.client.Update(context.TODO(), clusterGateway); err != nil {
		return err
	}
	return nil
}

func (c *ClusterGatewayInstaller) ensureClusterProxySecrets(config *proxyv1alpha1.ClusterGatewayConfiguration) error {
	if config.Spec.Egress.Type != proxyv1alpha1.EgressTypeClusterProxy {
		return nil
	}
	proxyClientCASecretName := config.Spec.Egress.ClusterProxy.Credentials.ProxyClientCASecretName
	err := cert.CopySecret(c.nativeClient,
		config.Spec.Egress.ClusterProxy.Credentials.Namespace, proxyClientCASecretName,
		config.Spec.InstallNamespace, proxyClientCASecretName)
	if err != nil {
		return errors.Wrapf(err, "failed copy secret %v", proxyClientCASecretName)
	}
	proxyClientSecretName := config.Spec.Egress.ClusterProxy.Credentials.ProxyClientSecretName
	err = cert.CopySecret(c.nativeClient,
		config.Spec.Egress.ClusterProxy.Credentials.Namespace, proxyClientSecretName,
		config.Spec.InstallNamespace, proxyClientSecretName)
	if err != nil {
		return errors.Wrapf(err, "failed copy secret %v", proxyClientSecretName)
	}
	return nil
}

func (c *ClusterGatewayInstaller) ensureSecretManagement(clusterAddon *addonv1alpha1.ClusterManagementAddOn, config *proxyv1alpha1.ClusterGatewayConfiguration) error {
	if config.Spec.SecretManagement.Type != proxyv1alpha1.SecretManagementTypeManagedServiceAccount {
		return nil
	}
	if _, err := c.mapper.KindFor(schema.GroupVersionResource{
		Group:    ocmauthv1alpha1.GroupVersion.Group,
		Version:  ocmauthv1alpha1.GroupVersion.Version,
		Resource: "managedserviceaccounts",
	}); err != nil {
		return fmt.Errorf("failed to discover ManagedServiceAccount resource in the cluster")
	}
	addonList := &addonv1alpha1.ManagedClusterAddOnList{}
	if err := c.client.List(context.TODO(), addonList); err != nil {
		return errors.Wrapf(err, "failed to list managed cluster addons")
	}
	clusterGatewayAddon := make([]*addonv1alpha1.ManagedClusterAddOn, 0)
	for _, addon := range addonList.Items {
		addon := addon
		if addon.Name == common.AddonName {
			clusterGatewayAddon = append(clusterGatewayAddon, &addon)
		}
	}
	for _, addon := range clusterGatewayAddon {
		managedServiceAccount := buildManagedServiceAccount(addon)
		if err := c.client.Create(context.TODO(), managedServiceAccount); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return errors.Wrapf(err, "failed to create managed serviceaccount")
			}
		}

		if err := c.copySecretForManagedServiceAccount(
			clusterAddon,
			config,
			addon.Namespace); err != nil {
			return errors.Wrapf(err, "failed to copy secret from managed serviceaccount")
		}
	}
	return nil
}

func (c *ClusterGatewayInstaller) copySecretForManagedServiceAccount(addon *addonv1alpha1.ClusterManagementAddOn, config *proxyv1alpha1.ClusterGatewayConfiguration, clusterName string) error {
	endpointType := clusterv1alpha1.ClusterEndpointTypeConst
	if config.Spec.Egress.Type == proxyv1alpha1.EgressTypeClusterProxy {
		endpointType = clusterv1alpha1.ClusterEndpointTypeClusterProxy
	}
	gatewaySecretNamespace := config.Spec.SecretNamespace
	secretName := config.Spec.SecretManagement.ManagedServiceAccount.Name

	secret, err := c.secretLister.Secrets(clusterName).
		Get(secretName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get token secret")
		}
		return nil
	}
	currentSecret, err := c.secretLister.Secrets(gatewaySecretNamespace).Get(clusterName)
	shouldCreate := false
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get the cluster secret")
		}
		shouldCreate = true
	}
	if shouldCreate {
		if _, err := c.nativeClient.CoreV1().Secrets(gatewaySecretNamespace).
			Create(context.TODO(),
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: gatewaySecretNamespace,
						Name:      clusterName,
						Labels: map[string]string{
							clusterv1alpha1.LabelKeyClusterCredentialType: string(clusterv1alpha1.CredentialTypeServiceAccountToken),
							clusterv1alpha1.LabelKeyClusterEndpointType:   string(endpointType),
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: addonv1alpha1.GroupVersion.String(),
								Kind:       "ClusterManagementAddOn",
								UID:        addon.UID,
								Name:       addon.Name,
							},
						},
					},
					Type: corev1.SecretTypeOpaque,
					Data: map[string][]byte{
						corev1.ServiceAccountRootCAKey: secret.Data[corev1.ServiceAccountRootCAKey],
						corev1.ServiceAccountTokenKey:  secret.Data[corev1.ServiceAccountTokenKey],
					},
				},
				metav1.CreateOptions{}); err != nil {
			return errors.Wrapf(err, "failed to create the cluster secret")
		}
	} else {
		if bytes.Equal(secret.Data[corev1.ServiceAccountTokenKey], currentSecret.Data[corev1.ServiceAccountTokenKey]) {
			return nil // no need for an update
		}
		currentSecret.Data[corev1.ServiceAccountRootCAKey] = secret.Data[corev1.ServiceAccountRootCAKey]
		currentSecret.Data[corev1.ServiceAccountTokenKey] = secret.Data[corev1.ServiceAccountTokenKey]
		if _, err := c.nativeClient.CoreV1().Secrets(gatewaySecretNamespace).
			Update(context.TODO(), currentSecret, metav1.UpdateOptions{}); err != nil {
			return errors.Wrapf(err, "failed to update the cluster secret")
		}
	}
	return nil
}

func newServiceAccount(addon *addonv1alpha1.ClusterManagementAddOn, namespace string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      common.AddonName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: addonv1alpha1.GroupVersion.String(),
					Kind:       "ClusterManagementAddOn",
					UID:        addon.UID,
					Name:       addon.Name,
				},
			},
		},
	}
}

const labelKeyClusterGatewayConfigurationGeneration = "proxy.open-cluster-management.io/configuration-generation"

func newClusterGatewayDeployment(addon *addonv1alpha1.ClusterManagementAddOn, config *proxyv1alpha1.ClusterGatewayConfiguration) *appsv1.Deployment {
	args := []string{
		"--secure-port=9443",
		"--secret-namespace=" + config.Spec.SecretNamespace,
		"--ocm-integration=true",
		"--tls-cert-file=/etc/server/tls.crt",
		"--tls-private-key-file=/etc/server/tls.key",
		"--feature-gates=HealthinessCheck=true",
	}
	volumes := []corev1.Volume{
		{
			Name: "server",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: SecretNameClusterGatewayTLSCert,
				},
			},
		},
	}
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "server",
			MountPath: "/etc/server/",
			ReadOnly:  true,
		},
	}
	if config.Spec.Egress.Type == proxyv1alpha1.EgressTypeClusterProxy {
		args = append(args,
			"--proxy-host="+config.Spec.Egress.ClusterProxy.ProxyServerHost,
			"--proxy-port="+strconv.Itoa(int(config.Spec.Egress.ClusterProxy.ProxyServerPort)),
			"--proxy-ca-cert=/etc/ca/ca.crt",
			"--proxy-cert=/etc/tls/tls.crt",
			"--proxy-key=/etc/tls/tls.key",
		)
		volumes = append(volumes,
			corev1.Volume{
				Name: "proxy-client-ca",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "proxy-server-ca",
					},
				},
			},
			corev1.Volume{
				Name: "proxy-client",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "proxy-client",
					},
				},
			},
		)
		volumeMounts = append(volumeMounts,
			corev1.VolumeMount{
				Name:      "proxy-client-ca",
				MountPath: "/etc/ca/",
			},
			corev1.VolumeMount{
				Name:      "proxy-client",
				MountPath: "/etc/tls/",
			},
		)
	}

	maxUnavailable := intstr.FromInt(1)
	maxSurge := intstr.FromInt(1)
	deploy := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: config.Spec.InstallNamespace,
			Name:      "gateway-deployment",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: addonv1alpha1.GroupVersion.String(),
					Kind:       "ClusterManagementAddOn",
					UID:        addon.UID,
					Name:       addon.Name,
				},
			},
			Labels: map[string]string{
				labelKeyClusterGatewayConfigurationGeneration: strconv.Itoa(int(config.Generation)),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					common.LabelKeyOpenClusterManagementAddon: common.AddonName,
				},
			},
			Replicas: pointer.Int32(3),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						common.LabelKeyOpenClusterManagementAddon: common.AddonName,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "apiserver",
							Image:           config.Spec.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args:            args,
							VolumeMounts:    volumeMounts,
						},
					},
					ServiceAccountName: common.AddonName,
					Volumes:            volumes,
				},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{

					MaxUnavailable: &maxUnavailable,
					MaxSurge:       &maxSurge,
				},
			},
		},
	}
	return deploy
}

func newClusterGatewayService(addon *addonv1alpha1.ClusterManagementAddOn, namespace string) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      common.AddonName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: addonv1alpha1.GroupVersion.String(),
					Kind:       "ClusterManagementAddOn",
					UID:        addon.UID,
					Name:       addon.Name,
				},
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				common.LabelKeyOpenClusterManagementAddon: common.AddonName,
			},
			Ports: []corev1.ServicePort{
				{
					Name: "https",
					Port: 9443,
				},
			},
		},
	}
}

func newAPIService(addon *addonv1alpha1.ClusterManagementAddOn, namespace string, verifyingCABundle []byte) *apiregistrationv1.APIService {
	return &apiregistrationv1.APIService{
		ObjectMeta: metav1.ObjectMeta{
			Name: "v1alpha1.cluster.core.oam.dev",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: addonv1alpha1.GroupVersion.String(),
					Kind:       "ClusterManagementAddOn",
					UID:        addon.UID,
					Name:       addon.Name,
				},
			},
		},
		Spec: apiregistrationv1.APIServiceSpec{
			Group:   "cluster.core.oam.dev",
			Version: "v1alpha1",
			Service: &apiregistrationv1.ServiceReference{
				Namespace: namespace,
				Name:      common.AddonName,
				Port:      pointer.Int32(9443),
			},
			GroupPriorityMinimum: 5000,
			VersionPriority:      10,
			CABundle:             verifyingCABundle,
		},
	}
}

func newAuthenticationRole(addon *addonv1alpha1.ClusterManagementAddOn, namespace string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "kube-system",
			Name:      "extension-apiserver-authentication-reader:cluster-gateway",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: addonv1alpha1.GroupVersion.String(),
					Kind:       "ClusterManagementAddOn",
					UID:        addon.UID,
					Name:       addon.Name,
				},
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind: "Role",
			Name: "extension-apiserver-authentication-reader",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Namespace: namespace,
				Name:      common.AddonName,
			},
		},
	}
}

func newSecretRole(addon *addonv1alpha1.ClusterManagementAddOn, secretNamespace string) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: secretNamespace,
			Name:      "cluster-gateway",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: addonv1alpha1.GroupVersion.String(),
					Kind:       "ClusterManagementAddOn",
					UID:        addon.UID,
					Name:       addon.Name,
				},
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"get", "list", "watch", "update"},
			},
		},
	}
}

func newSecretRoleBinding(addon *addonv1alpha1.ClusterManagementAddOn, namespace, secretNamespace string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: secretNamespace,
			Name:      "cluster-gateway",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: addonv1alpha1.GroupVersion.String(),
					Kind:       "ClusterManagementAddOn",
					UID:        addon.UID,
					Name:       addon.Name,
				},
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind: "Role",
			Name: "cluster-gateway",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Namespace: namespace,
				Name:      common.AddonName,
			},
		},
	}
}
func newAPFClusterRole(addon *addonv1alpha1.ClusterManagementAddOn) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "apiserver-aggregation:cluster-gateway",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: addonv1alpha1.GroupVersion.String(),
					Kind:       "ClusterManagementAddOn",
					UID:        addon.UID,
					Name:       addon.Name,
				},
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"cluster.open-cluster-management.io"},
				Resources: []string{"managedclusters"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"namespaces"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"admissionregistration.k8s.io"},
				Resources: []string{"mutatingwebhookconfigurations", "validatingwebhookconfigurations"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"flowcontrol.apiserver.k8s.io"},
				Resources: []string{"prioritylevelconfigurations", "flowschemas"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"authorization.k8s.io"},
				Resources: []string{"subjectaccessreviews"},
				Verbs:     []string{"*"},
			},
		},
	}
}

func newAPFClusterRoleBinding(addon *addonv1alpha1.ClusterManagementAddOn, namespace string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "apiserver-aggregation:cluster-gateway",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: addonv1alpha1.GroupVersion.String(),
					Kind:       "ClusterManagementAddOn",
					UID:        addon.UID,
					Name:       addon.Name,
				},
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind: "ClusterRole",
			Name: "apiserver-aggregation:cluster-gateway",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Namespace: namespace,
				Name:      common.AddonName,
			},
		},
	}
}

func buildManagedServiceAccount(addon *addonv1alpha1.ManagedClusterAddOn) *ocmauthv1alpha1.ManagedServiceAccount {
	return &ocmauthv1alpha1.ManagedServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "authentication.open-cluster-management.io/v1alpha1",
			Kind:       "ManagedServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: addon.Namespace,
			Name:      common.AddonName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: addonv1alpha1.GroupVersion.String(),
					Kind:       "ManagedClusterAddOn",
					UID:        addon.UID,
					Name:       addon.Name,
				},
			},
		},
		Spec: ocmauthv1alpha1.ManagedServiceAccountSpec{
			Rotation: ocmauthv1alpha1.ManagedServiceAccountRotation{
				Enabled: true,
				Validity: metav1.Duration{
					Duration: time.Hour * 24 * 180,
				},
			},
		},
	}
}
