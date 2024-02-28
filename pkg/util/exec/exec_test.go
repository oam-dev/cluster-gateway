package exec

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/apis/clientauthentication"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var (
	testClusterName = "my-cluster"
)

func TestIssueClusterCredential(t *testing.T) {
	t0 := time.Now()

	cases := map[string]struct {
		clusterName   string
		execConfig    *clientcmdapi.ExecConfig
		expected      *clientauthentication.ExecCredential
		expectedError string
		setup         func(t *testing.T)
	}{
		"missing cluster name": {
			expectedError: "cluster name not provided",
		},

		"missing exec config": {
			clusterName:   testClusterName,
			expectedError: "exec config not provided",
		},

		"missing command on exec config": {
			clusterName:   testClusterName,
			execConfig:    &clientcmdapi.ExecConfig{},
			expectedError: "missing \"command\" property on exec config object",
		},

		"successfully issuing a token": {
			clusterName: testClusterName,
			execConfig: &clientcmdapi.ExecConfig{
				APIVersion: "client.authentication.k8s.io/v1",
				Command:    "echo",
				Args: []string{"-n", `{
  "apiVersion": "client.authentication.k8s.io/v1",
  "kind": "ExecCredential",
  "status": {
    "token": "testToken"
  }
}`},
			},
			expected: &clientauthentication.ExecCredential{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "client.authentication.k8s.io/v1",
					Kind:       "ExecCredential",
				},
				Status: &clientauthentication.ExecCredentialStatus{
					Token: "testToken",
				},
			},
		},

		"credential HIT from cache": {
			setup: func(t *testing.T) {
				credentials.Store(testClusterName, &clientauthentication.ExecCredential{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "client.authentication.k8s.io/v1",
						Kind:       "ExecCredential",
					},
					Status: &clientauthentication.ExecCredentialStatus{
						ExpirationTimestamp: buildMetav1Time(t, t0.Add(time.Hour)),
						Token:               "testToken",
					},
				})
			},
			clusterName: testClusterName,
			execConfig: &clientcmdapi.ExecConfig{
				APIVersion: "client.authentication.k8s.io/v1",
				Command:    "should_be_ignored",
			},
			expected: &clientauthentication.ExecCredential{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "client.authentication.k8s.io/v1",
					Kind:       "ExecCredential",
				},
				Status: &clientauthentication.ExecCredentialStatus{
					ExpirationTimestamp: buildMetav1Time(t, t0.Add(time.Hour)),
					Token:               "testToken",
				},
			},
		},

		"credential MISS from cache": {
			setup: func(t *testing.T) {
				credentials.Store(testClusterName, &clientauthentication.ExecCredential{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "client.authentication.k8s.io/v1",
						Kind:       "ExecCredential",
					},
					Status: &clientauthentication.ExecCredentialStatus{
						ExpirationTimestamp: buildMetav1Time(t, t0),
						Token:               "oldToken",
					},
				})
			},
			clusterName: testClusterName,
			execConfig: &clientcmdapi.ExecConfig{
				APIVersion: "client.authentication.k8s.io/v1",
				Command:    "echo",
				Args: []string{
					"-n",
					fmt.Sprintf(`{
  "apiVersion": "client.authentication.k8s.io/v1",
  "kind": "ExecCredential",
  "status": {
    "expirationTimestamp": %q,
    "token": "newToken"
  }
}`, t0.Add(24*time.Hour).UTC().Format(time.RFC3339)),
				},
			},
			expected: &clientauthentication.ExecCredential{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "client.authentication.k8s.io/v1",
					Kind:       "ExecCredential",
				},
				Status: &clientauthentication.ExecCredentialStatus{
					ExpirationTimestamp: buildMetav1Time(t, t0.Add(24*time.Hour)),
					Token:               "newToken",
				},
			},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			cleanAllCache(t)

			if tt.setup != nil {
				tt.setup(t)
			}

			cred, err := IssueClusterCredential(tt.clusterName, tt.execConfig)
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.EqualError(t, err, tt.expectedError)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, cred)
		})
	}
}

func buildMetav1Time(t *testing.T, tm time.Time) *metav1.Time {
	t.Helper()

	copied, err := time.Parse(time.RFC3339, tm.Format(time.RFC3339))
	assert.NoError(t, err, "failed to parse time to RFC3339: %s", err)

	return &metav1.Time{Time: copied}
}

func cleanAllCache(t *testing.T) {
	t.Helper()

	credentials.Range(func(key, value any) bool {
		credentials.Delete(key)
		return true
	})
}
