package v1alpha1

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"

	"github.com/oam-dev/cluster-gateway/pkg/config"
)

func TestClusterGatewayStorageMeta(t *testing.T) {
	cg := &ClusterGateway{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}

	assert.Equal(t, &cg.ObjectMeta, cg.GetObjectMeta())
	assert.False(t, cg.NamespaceScoped())
	assert.Equal(t, &ClusterGateway{}, cg.New())
	assert.Equal(t, &ClusterGatewayList{}, cg.NewList())
	assert.True(t, cg.IsStorageVersion())
	assert.Equal(t, "clustergateway", cg.GetSingularName())
	assert.Equal(t, schema.GroupVersionResource{
		Group:    config.MetaApiGroupName,
		Version:  config.MetaApiVersionName,
		Resource: config.MetaApiResourceName,
	}, cg.GetGroupVersionResource())
	cg.Destroy()

	list := &ClusterGatewayList{}
	assert.Equal(t, &list.ListMeta, list.GetListMeta())
}

func TestValidateClusterGateway(t *testing.T) {
	validSpec := ClusterGatewaySpec{
		Provider: "myProvider",
		Access: ClusterAccess{
			Endpoint: &ClusterEndpoint{
				Type: ClusterEndpointTypeConst,
				Const: &ClusterEndpointConst{
					Address:  "https://example.com:6443",
					Insecure: pointer.Bool(true),
				},
			},
			Credential: &ClusterAccessCredential{
				Type:                CredentialTypeServiceAccountToken,
				ServiceAccountToken: "myToken",
			},
		},
	}

	cases := []struct {
		name       string
		mutateSpec func(spec *ClusterGatewaySpec)
		wantErrs   int
	}{
		{
			name:     "valid spec has no errors",
			wantErrs: 0,
		},
		{
			name: "missing provider",
			mutateSpec: func(spec *ClusterGatewaySpec) {
				spec.Provider = ""
			},
			wantErrs: 1,
		},
		{
			name: "non-https endpoint scheme",
			mutateSpec: func(spec *ClusterGatewaySpec) {
				spec.Access.Endpoint.Const.Address = "http://example.com:6443"
			},
			wantErrs: 1,
		},
		{
			name: "missing caBundle for non-insecure endpoint",
			mutateSpec: func(spec *ClusterGatewaySpec) {
				spec.Access.Endpoint.Const.Insecure = nil
			},
			wantErrs: 1,
		},
		{
			name: "unsupported credential type",
			mutateSpec: func(spec *ClusterGatewaySpec) {
				spec.Access.Credential.Type = "Bogus"
			},
			wantErrs: 1,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			spec := *validSpec.DeepCopy()
			if c.mutateSpec != nil {
				c.mutateSpec(&spec)
			}
			cg := &ClusterGateway{Spec: spec}
			errs := cg.Validate(context.TODO())
			assert.Len(t, errs, c.wantErrs)
		})
	}
}
