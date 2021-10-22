package main

import (
	"context"
	"fmt"
	multicluster "github.com/oam-dev/cluster-gateway/pkg/apis/cluster/transport"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var kubeconfig string
var clusterName string

func main() {

	cmd := cobra.Command{
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
			if err != nil {
				return err
			}
			cfg.Wrap(multicluster.NewClusterGatewayRoundTripper)

			// Native kubernetes client example
			nativeClient := kubernetes.NewForConfigOrDie(cfg)
			defaultNs, err := nativeClient.CoreV1().Namespaces().Get(
				multicluster.WithMultiClusterContext(context.TODO(), clusterName),
				"default",
				metav1.GetOptions{})
			fmt.Printf("Native client get default namespace: %v\n", defaultNs)

			// Controller-runtime client example
			controllerRuntimeClient, err := client.New(cfg, client.Options{})
			if err != nil {
				panic(err)
			}
			ns := &corev1.Namespace{}
			err = controllerRuntimeClient.Get(
				multicluster.WithMultiClusterContext(context.TODO(), clusterName),
				types.NamespacedName{Name: "default"},
				ns)
			if err != nil {
				panic(err)
			}
			fmt.Printf("Controller-runtime client get default namespace: %v\n", ns)
			return nil
		},
	}

	cmd.Flags().StringVarP(&kubeconfig, "kubeconfig", "", "", "the client kubeconfig")
	cmd.Flags().StringVarP(&clusterName, "cluster-name", "", "", "the target cluster name")

	if err := cmd.Execute(); err != nil {
		panic(err)
	}
}
