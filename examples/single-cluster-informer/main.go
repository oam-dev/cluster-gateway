package main

import (
	"context"
	"fmt"
	"time"

	multicluster "github.com/oam-dev/cluster-gateway/pkg/apis/cluster/transport"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	controllers "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
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
			cfg.Wrap(multicluster.NewEnhanceClusterGatewayRoundTripper(clusterName).NewRoundTripper)

			// Native kubernetes client informer
			nativeClient := kubernetes.NewForConfigOrDie(cfg)

			sharedInformer := informers.NewSharedInformerFactory(nativeClient, 0)
			podInformer := sharedInformer.Core().V1().Pods().Informer()

			fmt.Printf("Native client cache pod info:\n")
			podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{AddFunc: addFunc})

			ctx, cancel := context.WithCancel(context.TODO())
			go sharedInformer.Start(ctx.Done())
			for !podInformer.HasSynced() {
				time.Sleep(time.Millisecond * 100)
			}
			cancel()

			// Controller-runtime client informer
			s := runtime.NewScheme()
			scheme.AddToScheme(s)

			mgr, err := controllers.NewManager(cfg, manager.Options{Scheme: s})

			podInformer2, err := mgr.GetCache().GetInformer(context.TODO(), &corev1.Pod{})
			if err != nil {
				return err
			}

			fmt.Printf("Controller-runtime cache pod info:\n")
			podInformer2.AddEventHandler(cache.ResourceEventHandlerFuncs{AddFunc: addFunc})

			ctx, cancel = context.WithCancel(context.TODO())
			go mgr.Start(ctx)
			for !podInformer2.HasSynced() {
				time.Sleep(time.Millisecond * 100)
			}
			cancel()

			return nil
		},
	}

	cmd.Flags().StringVarP(&kubeconfig, "kubeconfig", "", "", "the client kubeconfig")
	cmd.Flags().StringVarP(&clusterName, "cluster-name", "", "", "the target cluster name")

	if err := cmd.Execute(); err != nil {
		panic(err)
	}
}

func addFunc(obj interface{}) {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return
	}
	if pod.Namespace == "kube-system" {
		fmt.Printf("%s\t%s\t%s\t%s\n", pod.Namespace, pod.Name, pod.Status.PodIP, pod.Status.HostIP)
	}
}
