package v1alpha1

import (
	"context"
	"testing"

	"github.com/oam-dev/cluster-gateway/pkg/config"
	"github.com/oam-dev/cluster-gateway/pkg/options"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes/fake"
	ocmclientfake "open-cluster-management.io/api/client/cluster/clientset/versioned/fake"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
)

var (
	testNamespace = "foo"
	testName      = "bar"
	testCAData    = "caData"
	testCertData  = "certData"
	testKeyData   = "keyData"
	testToken     = "token"
	testEndpoint  = "https://localhost:443"
)

func init() {
	initClient.Do(func() {})
}

func TestConvertSecretToGateway(t *testing.T) {
	cases := []struct {
		name            string
		inputSecret     *corev1.Secret
		expectedFailure bool
		expected        *ClusterGateway
	}{
		{
			name: "missing credential type label should fail",
			inputSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNamespace,
					Name:      testName,
				},
				Data: map[string][]byte{},
			},
			expectedFailure: true,
		},
		{
			name: "service-account token conversion",
			inputSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNamespace,
					Name:      testName,
					Labels: map[string]string{
						LabelKeyClusterCredentialType: string(CredentialTypeServiceAccountToken),
					},
				},
				Data: map[string][]byte{
					"ca.crt":   []byte(testCAData),
					"token":    []byte(testToken),
					"endpoint": []byte(testEndpoint),
				},
			},
			expected: &ClusterGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name: testName,
				},
				Spec: ClusterGatewaySpec{
					Access: ClusterAccess{
						Credential: &ClusterAccessCredential{
							Type:                CredentialTypeServiceAccountToken,
							ServiceAccountToken: testToken,
						},
						Endpoint: &ClusterEndpoint{
							Type: ClusterEndpointTypeConst,
							Const: &ClusterEndpointConst{
								CABundle: []byte(testCAData),
								Address:  testEndpoint,
							},
						},
					},
				},
			},
		},
		{
			name: "x509 certificate conversion",
			inputSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNamespace,
					Name:      testName,
					Labels: map[string]string{
						LabelKeyClusterCredentialType: string(CredentialTypeX509Certificate),
					},
				},
				Data: map[string][]byte{
					"ca.crt":   []byte(testCAData),
					"tls.crt":  []byte(testCertData),
					"tls.key":  []byte(testKeyData),
					"endpoint": []byte(testEndpoint),
				},
			},
			expected: &ClusterGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name: testName,
				},
				Spec: ClusterGatewaySpec{
					Access: ClusterAccess{
						Credential: &ClusterAccessCredential{
							Type: CredentialTypeX509Certificate,
							X509: &X509{
								Certificate: []byte(testCertData),
								PrivateKey:  []byte(testKeyData),
							},
						},
						Endpoint: &ClusterEndpoint{
							Type: ClusterEndpointTypeConst,
							Const: &ClusterEndpointConst{
								CABundle: []byte(testCAData),
								Address:  testEndpoint,
							},
						},
					},
				},
			},
		},
		{
			name: "cluster proxy egress conversion",
			inputSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNamespace,
					Name:      testName,
					Labels: map[string]string{
						LabelKeyClusterCredentialType: string(CredentialTypeX509Certificate),
						LabelKeyClusterEndpointType:   string(ClusterEndpointTypeClusterProxy),
					},
				},
				Data: map[string][]byte{
					"ca.crt":   []byte(testCAData),
					"tls.crt":  []byte(testCertData),
					"tls.key":  []byte(testKeyData),
					"endpoint": []byte(testEndpoint),
				},
			},
			expected: &ClusterGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name: testName,
				},
				Spec: ClusterGatewaySpec{
					Access: ClusterAccess{
						Credential: &ClusterAccessCredential{
							Type: CredentialTypeX509Certificate,
							X509: &X509{
								Certificate: []byte(testCertData),
								PrivateKey:  []byte(testKeyData),
							},
						},
						Endpoint: &ClusterEndpoint{
							Type: ClusterEndpointTypeClusterProxy,
						},
					},
				},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gw, err := convertFromSecret(c.inputSecret)
			if c.expectedFailure {
				assert.True(t, err != nil)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, c.expected, gw)
		})
	}
}

func TestConvertSecretAndClusterToGateway(t *testing.T) {
	cases := []struct {
		name            string
		inputSecret     *corev1.Secret
		inputCluster    *clusterv1.ManagedCluster
		expectedFailure bool
		expected        *ClusterGateway
	}{
		{
			name: "x509 certificate conversion",
			inputSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNamespace,
					Name:      testName,
					Labels: map[string]string{
						LabelKeyClusterCredentialType: string(CredentialTypeX509Certificate),
					},
				},
				Data: map[string][]byte{
					"tls.crt": []byte(testCertData),
					"tls.key": []byte(testKeyData),
				},
			},
			inputCluster: &clusterv1.ManagedCluster{
				Spec: clusterv1.ManagedClusterSpec{

					ManagedClusterClientConfigs: []clusterv1.ClientConfig{
						{
							URL:      testEndpoint,
							CABundle: []byte(testCAData),
						},
					},
					HubAcceptsClient: true,
				},
			},
			expected: &ClusterGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name: testName,
				},
				Spec: ClusterGatewaySpec{
					Access: ClusterAccess{
						Credential: &ClusterAccessCredential{
							Type: CredentialTypeX509Certificate,
							X509: &X509{
								Certificate: []byte(testCertData),
								PrivateKey:  []byte(testKeyData),
							},
						},
						Endpoint: &ClusterEndpoint{
							Type: ClusterEndpointTypeConst,
							Const: &ClusterEndpointConst{
								CABundle: []byte(testCAData),
								Address:  testEndpoint,
							},
						},
					},
				},
			},
		},
		{
			name: "service-account token conversion",
			inputSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: testNamespace,
					Name:      testName,
					Labels: map[string]string{
						LabelKeyClusterCredentialType: string(CredentialTypeServiceAccountToken),
					},
				},
				Data: map[string][]byte{
					"token":  []byte(testToken),
					"ca.crt": []byte("should be overrided"),
				},
			},
			inputCluster: &clusterv1.ManagedCluster{
				Spec: clusterv1.ManagedClusterSpec{

					ManagedClusterClientConfigs: []clusterv1.ClientConfig{
						{
							URL:      testEndpoint,
							CABundle: []byte(testCAData),
						},
					},
					HubAcceptsClient: true,
				},
			},
			expected: &ClusterGateway{
				ObjectMeta: metav1.ObjectMeta{
					Name: testName,
				},
				Spec: ClusterGatewaySpec{
					Access: ClusterAccess{
						Credential: &ClusterAccessCredential{
							Type:                CredentialTypeServiceAccountToken,
							ServiceAccountToken: testToken,
						},
						Endpoint: &ClusterEndpoint{
							Type: ClusterEndpointTypeConst,
							Const: &ClusterEndpointConst{
								CABundle: []byte(testCAData),
								Address:  testEndpoint,
							},
						},
					},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gw, err := convertFromManagedClusterAndSecret(c.inputCluster, c.inputSecret)
			if c.expectedFailure {
				assert.True(t, err != nil)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, c.expected, gw)
		})
	}
}

func TestGetClusterGateway(t *testing.T) {
	options.OCMIntegration = false
	config.SecretNamespace = testNamespace
	input := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      testName,
			Labels: map[string]string{
				LabelKeyClusterCredentialType: string(CredentialTypeServiceAccountToken),
			},
		},
		Data: map[string][]byte{
			"ca.crt":   []byte(testCAData),
			"token":    []byte(testToken),
			"endpoint": []byte(testEndpoint),
		},
	}
	fakeKubeClient := fake.NewSimpleClientset(input)
	setClient(fakeKubeClient, nil)
	expected, err := convertFromSecret(input)
	storage := &ClusterGateway{}
	gwRaw, err := storage.Get(context.TODO(), testName, &metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, expected, gwRaw)
}

func TestListClusterGateway(t *testing.T) {
	options.OCMIntegration = false
	config.SecretNamespace = testNamespace
	input := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      testName,
			Labels: map[string]string{
				LabelKeyClusterCredentialType: string(CredentialTypeX509Certificate),
			},
		},
		Data: map[string][]byte{
			"ca.crt":   []byte(testCAData),
			"tls.crt":  []byte(testCertData),
			"tls.key":  []byte(testKeyData),
			"endpoint": []byte(testEndpoint),
		},
	}
	fakeKubeClient := fake.NewSimpleClientset(input)
	setClient(fakeKubeClient, nil)

	storage := &ClusterGateway{}
	gws, err := storage.List(context.TODO(), &internalversion.ListOptions{})
	require.Equal(t, 1, len(gws.(*ClusterGatewayList).Items))
	assert.NoError(t, err)
	expected, err := convertFromSecret(input)
	assert.NoError(t, err)
	gw := gws.(*ClusterGatewayList).Items[0]
	assert.Equal(t, expected, &gw)
}

func TestListHybridClusterGateway(t *testing.T) {
	testNoClusterName := "no cluster connected"
	inputWithCluster := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      testName,
			Labels: map[string]string{
				LabelKeyClusterCredentialType: string(CredentialTypeX509Certificate),
			},
		},
		Data: map[string][]byte{
			"ca.crt":   []byte(testCAData),
			"tls.crt":  []byte(testCertData),
			"tls.key":  []byte(testKeyData),
			"endpoint": []byte(testEndpoint),
		},
	}
	inputNoCluster := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      testNoClusterName,
			Labels: map[string]string{
				LabelKeyClusterCredentialType: string(CredentialTypeX509Certificate),
			},
		},
		Data: map[string][]byte{
			"ca.crt":   []byte(testCAData),
			"tls.crt":  []byte(testCertData),
			"tls.key":  []byte(testKeyData),
			"endpoint": []byte(testEndpoint),
		},
	}
	inputDummy := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      "dummy",
			Labels:    map[string]string{},
		},
		Data: map[string][]byte{},
	}
	cluster := &clusterv1.ManagedCluster{
		Spec: clusterv1.ManagedClusterSpec{

			ManagedClusterClientConfigs: []clusterv1.ClientConfig{
				{
					URL:      testEndpoint,
					CABundle: []byte(testCAData),
				},
			},
			HubAcceptsClient: true,
		},
	}
	fakeKubeClient := fake.NewSimpleClientset(inputWithCluster, inputNoCluster, inputDummy)
	fakeOcmClient := ocmclientfake.NewSimpleClientset(cluster)
	setClient(fakeKubeClient, fakeOcmClient)

	storage := &ClusterGateway{}
	gws, err := storage.List(context.TODO(), &internalversion.ListOptions{})
	require.NoError(t, err)
	require.Equal(t, 2, len(gws.(*ClusterGatewayList).Items))
	expectedNames := sets.NewString(testName, testNoClusterName)
	actualNames := sets.NewString()
	for _, gw := range gws.(*ClusterGatewayList).Items {
		actualNames.Insert(gw.Name)
	}
	assert.Equal(t, expectedNames, actualNames)
}
