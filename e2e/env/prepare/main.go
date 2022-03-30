package main

import (
	"flag"
	"fmt"
	"github.com/oam-dev/cluster-gateway/pkg/apis/cluster/v1alpha1"
	"github.com/oam-dev/cluster-gateway/pkg/common"
	"os"
	"path/filepath"

	"github.com/ghodss/yaml"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

func main() {

	var clusterName string
	var secretNamespace string
	var dryRun bool

	flag.StringVar(&secretNamespace, "secret-namespace", "open-cluster-management-credentials",
		"Namespace of the cluster secret.")
	flag.StringVar(&clusterName, "cluster-name", "loopback",
		"Target name of the secret.")
	flag.BoolVar(&dryRun, "dry-run", false,
		"Whether to dry run")
	flag.Parse()

	kubeconfigPath := os.Getenv("KUBECONFIG")
	if len(kubeconfigPath) == 0 {
		kubeconfigPath = filepath.Join(os.Getenv("HOME"), ".kube", "config")
	}
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		klog.Fatal(err)
	}

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: secretNamespace,
			Name:      clusterName,
			Labels:    map[string]string{},
		},
		Data: map[string][]byte{
			"ca.crt":   restConfig.CAData,
			"endpoint": []byte("https://kubernetes.default.svc.cluster.local:443"),
		},
	}
	if len(restConfig.BearerToken) > 0 {
		// TODO
	} else {
		secret.Labels[common.LabelKeyClusterCredentialType] = string(v1alpha1.CredentialTypeX509Certificate)
		secret.Data["tls.crt"] = restConfig.CertData
		secret.Data["tls.key"] = restConfig.KeyData
	}

	secretYamlData, err := yaml.Marshal(secret)
	if err != nil {
		klog.Fatal(err)
	}

	fmt.Print(string(secretYamlData))
}
