package main

/*
Copyright 2021 The KubeVela Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import (
	"context"
	"fmt"
	"os"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/oam-dev/cluster-gateway/pkg/apis/cluster/v1alpha1"
)

const (
	FlagAPIServiceName    = "target-APIService"
	FlagSecretName        = "secret-name"
	FlagSecretNamespace   = "secret-namespace"
	FlagSecretCABundleKey = "secret-ca-bundle-key"
)

func buildSchemeOrDie() *runtime.Scheme {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		fmt.Printf("build client-go scheme error: %v\n", err)
		os.Exit(1)
	}
	if err := apiregistrationv1.AddToScheme(scheme); err != nil {
		fmt.Printf("build api-registration scheme error: %v\n", err)
		os.Exit(1)
	}
	return scheme
}

func main() {
	var APIServiceName string
	var secretName string
	var secretNamespace string
	var secretCABundleKey string
	cmd := &cobra.Command{
		Use: "patch",
		Short: "patch APIService CABundle from given secret",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client.New(config.GetConfigOrDie(), client.Options{Scheme: buildSchemeOrDie()})
			if err != nil {
				return errors.Wrapf(err, "get k8s client error")
			}
			ctx := context.Background()
			secret := &v1.Secret{}
			if err = c.Get(ctx, types.NamespacedName{Namespace: secretNamespace, Name: secretName}, secret); err != nil {
				return errors.Wrapf(err, "failed to get source secret")
			}
			apiService := &apiregistrationv1.APIService{}
			if err = c.Get(ctx, types.NamespacedName{Name: APIServiceName}, apiService); err != nil {
				return errors.Wrapf(err, "failed to get APIService")
			}
			caBundle, ok := secret.Data[secretCABundleKey]
			if !ok {
				return fmt.Errorf("failed to find caBundle in secret(%s/%s), key: %s", secretNamespace, secretName, secretCABundleKey)
			}
			apiService.Spec.InsecureSkipTLSVerify = false
			apiService.Spec.CABundle = caBundle
			if err = c.Update(ctx, apiService); err != nil {
				return errors.Wrapf(err, "failed to update APIService")
			}
			fmt.Printf("successfully update APIService %s caBundle: \n%s\n", APIServiceName, caBundle)
			return nil
		},
	}
	gv := v1alpha1.SchemeGroupVersion
	apiServiceName := gv.Version + "." + gv.Group
	cmd.Flags().StringVar(&APIServiceName, FlagAPIServiceName, apiServiceName, "specify the target APIService to patch caBundle")
	cmd.Flags().StringVar(&secretName, FlagSecretName, "", "specify the source secret name")
	cmd.Flags().StringVar(&secretNamespace, FlagSecretNamespace, "", "specify the source secret namespace")
	cmd.Flags().StringVar(&secretCABundleKey, FlagSecretCABundleKey, "ca", "specify the CABundle key in source secret")
	if err := cmd.Execute(); err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
}
