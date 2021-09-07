package main

import (
	"context"
	"fmt"
	"github.com/oam-dev/cluster-gateway/pkg/apis/cluster/v1alpha1"
	"github.com/oam-dev/cluster-gateway/pkg/generated/clientset/versioned"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/pointer"
)

var kubeconfig string

func main() {
	cmd := cobra.Command{
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
			if err != nil {
				return err
			}
			gatewayClient := versioned.NewForConfigOrDie(cfg)
			_, err = gatewayClient.ClusterV1alpha1().ClusterGateways().Create(
				context.TODO(),
				&v1alpha1.ClusterGateway{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
					Spec: v1alpha1.ClusterGatewaySpec{
						Provider: "foo",
						Access: v1alpha1.ClusterAccess{
							Endpoint: cfg.Host,
							CABundle: cfg.CAData,
							Insecure: pointer.BoolPtr(true),
							Credential: &v1alpha1.ClusterAccessCredential{
								Type:                v1alpha1.CredentialTypeServiceAccountToken,
								ServiceAccountToken: "abc",
							},
						},
					},
				},
				metav1.CreateOptions{})
			if err != nil {
				return err
			}
			testClusterClient := gatewayClient.ClusterV1alpha1().ClusterGateways().RESTClient("test")
			resp, err := testClusterClient.Get().AbsPath("/healthz").DoRaw(context.TODO())
			if err != nil {
				return err
			}
			fmt.Printf("Proxied Response: %v\n", string(resp))
			return nil
		},
	}

	cmd.Flags().StringVarP(&kubeconfig, "kubeconfig", "", "", "the client kubeconfig")
	if err := cmd.Execute(); err != nil {
		panic(err)
	}
}
