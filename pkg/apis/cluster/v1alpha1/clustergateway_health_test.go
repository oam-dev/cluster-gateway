package v1alpha1

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/component-base/featuregate/testing"

	"github.com/oam-dev/cluster-gateway/pkg/common"
	"github.com/oam-dev/cluster-gateway/pkg/config"
	"github.com/oam-dev/cluster-gateway/pkg/featuregates"
	"github.com/oam-dev/cluster-gateway/pkg/util/cert"
	"github.com/oam-dev/cluster-gateway/pkg/util/singleton"
)

func TestClusterGatewayHealthGet(t *testing.T) {
	cases := []struct {
		name            string
		parent          rest.Getter
		expectedFailure bool
	}{
		{
			name:            "no parent storage wired",
			parent:          nil,
			expectedFailure: true,
		},
		{
			name:            "parent lookup fails",
			parent:          &fakeParentStorage{err: errors.New("boom")},
			expectedFailure: true,
		},
		{
			name: "parent lookup succeeds",
			parent: &fakeParentStorage{obj: &ClusterGateway{
				ObjectMeta: metav1.ObjectMeta{Name: testName},
			}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			health := &ClusterGatewayHealth{Parent: c.parent}
			obj, err := health.Get(context.TODO(), testName, &metav1.GetOptions{})
			if c.expectedFailure {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, testName, obj.(*ClusterGateway).Name)
		})
	}
}

func TestClusterGatewayHealthUpdate(t *testing.T) {
	k8stesting.SetFeatureGateDuringTest(t, feature.DefaultMutableFeatureGate, featuregates.HealthinessCheck, true)

	prevNamespace := config.SecretNamespace
	prevSecretControl := singleton.GetSecretControl()
	prevKubeClient := singleton.GetKubeClient()
	t.Cleanup(func() {
		config.SecretNamespace = prevNamespace
		singleton.SetSecretControl(prevSecretControl)
		singleton.SetKubeClient(prevKubeClient)
	})

	config.SecretNamespace = testNamespace
	singleton.SetSecretControl(nil)
	health := &ClusterGatewayHealth{}
	_, _, err := health.Update(context.TODO(), testName, rest.DefaultUpdatedObjectInfo(&ClusterGateway{}), nil, nil, false, &metav1.UpdateOptions{})
	assert.EqualError(t, err, "loopback clients are not inited")

	input := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      testName,
			Labels: map[string]string{
				common.LabelKeyClusterCredentialType: string(CredentialTypeServiceAccountToken),
			},
		},
		Data: map[string][]byte{
			"token":    []byte(testToken),
			"endpoint": []byte(testEndpoint),
		},
	}
	fakeKubeClient := fake.NewSimpleClientset(input)
	singleton.SetSecretControl(cert.NewDirectApiSecretControl(testNamespace, fakeKubeClient))
	singleton.SetKubeClient(fakeKubeClient)

	updating := &ClusterGateway{Status: ClusterGatewayStatus{
		Healthy:       true,
		HealthyReason: HealthyReasonTypeConnectionTimeout,
	}}
	obj, created, err := health.Update(context.TODO(), testName, rest.DefaultUpdatedObjectInfo(updating), nil, nil, false, &metav1.UpdateOptions{})
	require.NoError(t, err)
	assert.False(t, created)
	gw := obj.(*ClusterGateway)
	assert.True(t, gw.Status.Healthy)
	assert.Equal(t, HealthyReasonTypeConnectionTimeout, gw.Status.HealthyReason)

	updated, err := fakeKubeClient.CoreV1().Secrets(testNamespace).Get(context.TODO(), testName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "true", updated.Annotations[AnnotationKeyClusterGatewayStatusHealthy])
	assert.Equal(t, string(HealthyReasonTypeConnectionTimeout), updated.Annotations[AnnotationKeyClusterGatewayStatusHealthyReason])
}
