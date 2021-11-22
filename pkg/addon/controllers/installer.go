package controllers

import (
	"context"
	"fmt"
	"strconv"
	"time"

	proxyv1alpha1 "github.com/oam-dev/cluster-gateway/pkg/apis/proxy/v1alpha1"
	"github.com/oam-dev/cluster-gateway/pkg/common"
	"github.com/oam-dev/cluster-gateway/pkg/event"
	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/library-go/pkg/operator/events"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	corev1lister "k8s.io/client-go/listers/core/v1"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"k8s.io/utils/pointer"
	"open-cluster-management.io/addon-framework/pkg/certrotation"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&addonv1alpha1.ClusterManagementAddOn{}).
		Watches(
			&source.Kind{
				Type: &proxyv1alpha1.ClusterGatewayConfiguration{},
			},
			&event.ClusterGatewayConfigurationHandler{
				Client: mgr.GetClient(),
			}).
		Complete(installer)
}

type ClusterGatewayInstaller struct {
	nativeClient kubernetes.Interface
	secretLister corev1lister.SecretLister
	caPair       *crypto.CA
	client       client.Client
	cache        cache.Cache
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
	if err := c.ensureProxySecrets(clusterGatewayConfiguration); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "failed to ensure required proxy client related credentials")
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
	namespace := clusterGatewayConfiguration.Spec.InstallNamespace
	targets := []client.Object{
		newServiceAccount(addon, namespace),
		newClusterGatewayService(addon, namespace),
		newClusterGatewayDeployment(addon, clusterGatewayConfiguration),
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

func (c *ClusterGatewayInstaller) ensureProxySecrets(config *proxyv1alpha1.ClusterGatewayConfiguration) error {
	if config.Spec.Egress.Type != proxyv1alpha1.EgressTypeClusterProxy {
		return nil
	}
	namespace := config.Spec.Egress.ClusterProxy.Credentials.Namespace
	proxyClientCASecret, err := c.nativeClient.CoreV1().
		Secrets(namespace).
		Get(context.TODO(), config.Spec.Egress.ClusterProxy.Credentials.ProxyClientCASecretName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed getting proxy ca secret")
	}
	proxyClientSecret, err := c.nativeClient.CoreV1().
		Secrets(namespace).
		Get(context.TODO(), config.Spec.Egress.ClusterProxy.Credentials.ProxyClientSecretName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed getting proxy ca secret")
	}
	proxyClientCASecret.Namespace = config.Spec.InstallNamespace
	proxyClientCASecret.ResourceVersion = ""
	proxyClientSecret.Namespace = config.Spec.InstallNamespace
	proxyClientSecret.ResourceVersion = ""
	if _, err := c.nativeClient.CoreV1().Secrets(config.Spec.InstallNamespace).
		Create(context.TODO(), proxyClientCASecret, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "failed creating CA secret")
		}
	}
	if _, err := c.nativeClient.CoreV1().Secrets(config.Spec.InstallNamespace).
		Create(context.TODO(), proxyClientSecret, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return errors.Wrapf(err, "failed creating CA secret")
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

func newClusterGatewayDeployment(addon *addonv1alpha1.ClusterManagementAddOn, config *proxyv1alpha1.ClusterGatewayConfiguration) *appsv1.Deployment {
	args := []string{
		"--secure-port=9443",
		"--secret-namespace=" + config.Spec.SecretNamespace,
		"--ocm-integration=true",
		"--tls-cert-file=/etc/server/tls.crt",
		"--tls-private-key-file=/etc/server/tls.key",
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

	deploy := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: config.Spec.InstallNamespace,
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
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					common.LabelKeyOpenClusterManagementAddon: common.AddonName,
				},
			},
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
			Name: "v1alpha1.gateway.proxy.open-cluster-management.io",
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
			Group:   "gateway.proxy.open-cluster-management.io",
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
				Verbs:     []string{"get", "list", "watch"},
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
