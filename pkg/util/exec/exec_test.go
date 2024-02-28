//go:build unix

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

		"missing command property within exec config": {
			clusterName:   testClusterName,
			execConfig:    &clientcmdapi.ExecConfig{},
			expectedError: "missing \"command\" property on exec config object",
		},

		"failed to run external command: command not found": {
			clusterName: testClusterName,
			execConfig: &clientcmdapi.ExecConfig{
				Command: "/path/to/command/not/found",
			},
			expectedError: "exec: executable /path/to/command/not/found not found",
		},

		"failed to run external command: finished with non-zero exit code": {
			clusterName: testClusterName,
			execConfig: &clientcmdapi.ExecConfig{
				APIVersion: "client.authentication.k8s.io/v1",
				Command:    "false",
			},
			expectedError: "exec: executable /usr/bin/false failed with exit code 1",
		},

		"missing API version in exec config": {
			clusterName: testClusterName,
			execConfig: &clientcmdapi.ExecConfig{
				Command: "true",
			},
			expectedError: `exec plugin: invalid apiVersion ""`,
		},

		"invalid API version in exec config": {
			clusterName: testClusterName,
			execConfig: &clientcmdapi.ExecConfig{
				APIVersion: "example.org/v1",
				Command:    "true",
			},
			expectedError: `exec plugin: invalid apiVersion "example.org/v1"`,
		},

		"invalid exec credential JSON": {
			clusterName: testClusterName,
			execConfig: &clientcmdapi.ExecConfig{
				APIVersion: "client.authentication.k8s.io/v1",
				Command:    "echo",
				Args:       []string{"-n", `[]`},
			},
			expectedError: "decoding stdout: couldn't get version/kind; json parse error: json: cannot unmarshal array into Go value of type struct { APIVersion string \"json:\\\"apiVersion,omitempty\\\"\"; Kind string \"json:\\\"kind,omitempty\\\"\" }",
		},

		"API version mismatch": {
			clusterName: testClusterName,
			execConfig: &clientcmdapi.ExecConfig{
				APIVersion: "client.authentication.k8s.io/v1",
				Command:    "echo",
				Args: []string{"-n", `{
  "apiVersion": "client.authentication.k8s.io/v1beta1",
  "kind": "ExecCredential",
  "status": {
    "token": "testToken"
  }
}`},
			},
			expectedError: "exec plugin is configured to use API version client.authentication.k8s.io/v1, plugin returned version client.authentication.k8s.io/v1beta1",
		},

		"missing status property on external command output": {
			clusterName: testClusterName,
			execConfig: &clientcmdapi.ExecConfig{
				APIVersion: "client.authentication.k8s.io/v1",
				Command:    "echo",
				Args:       []string{"-n", `{"apiVersion": "client.authentication.k8s.io/v1", "kind": "ExecCredential"}`},
			},
			expectedError: "exec plugin didn't return a status field",
		},

		"missing any auth credential on status": {
			clusterName: testClusterName,
			execConfig: &clientcmdapi.ExecConfig{
				APIVersion: "client.authentication.k8s.io/v1",
				Command:    "echo",
				Args:       []string{"-n", `{"apiVersion": "client.authentication.k8s.io/v1", "kind": "ExecCredential", "status": {}}`},
			},
			expectedError: "exec plugin didn't return a token or cert/key pair",
		},

		"has cert but no private key": {
			clusterName: testClusterName,
			execConfig: &clientcmdapi.ExecConfig{
				APIVersion: "client.authentication.k8s.io/v1",
				Command:    "echo",
				Args:       []string{"-n", `{"apiVersion": "client.authentication.k8s.io/v1", "kind": "ExecCredential", "status": {"clientCertificateData": "certData"}}`},
			},
			expectedError: "exec plugin returned only certificate or key, not both",
		},

		"invalid exec credential item on cache": {
			setup: func(t *testing.T) {
				credentials.Store(testClusterName, "invalid exec credential")
			},
			clusterName: testClusterName,
			execConfig: &clientcmdapi.ExecConfig{
				APIVersion: "client.authentication.k8s.io/v1",
				Command:    "should_be_ignored",
			},
			expectedError: "failed to convert item in cache to ExecCredential",
		},

		"MISS credential from cache, should issue a new credential": {
			clusterName: testClusterName,
			execConfig: &clientcmdapi.ExecConfig{
				APIVersion: "client.authentication.k8s.io/v1",
				Env: []clientcmdapi.ExecEnvVar{
					{Name: "TOKEN", Value: "testToken"},
				},
				Command: "echo",
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

		"HIT credential from cache": {
			setup: func(t *testing.T) {
				credentials.Store(testClusterName, &clientauthentication.ExecCredential{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "client.authentication.k8s.io/v1",
						Kind:       "ExecCredential",
					},
					Status: &clientauthentication.ExecCredentialStatus{
						ExpirationTimestamp: &metav1.Time{Time: t0.Add(time.Hour).Local().Truncate(time.Second)},
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
					ExpirationTimestamp: &metav1.Time{Time: t0.Add(time.Hour).Local().Truncate(time.Second)},
					Token:               "testToken",
				},
			},
		},

		"expired credential on cache, should issue a new credential": {
			setup: func(t *testing.T) {
				credentials.Store(testClusterName, &clientauthentication.ExecCredential{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "client.authentication.k8s.io/v1",
						Kind:       "ExecCredential",
					},
					Status: &clientauthentication.ExecCredentialStatus{
						ExpirationTimestamp: &metav1.Time{Time: t0},
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
}`, t0.Add(24*time.Hour).Format(time.RFC3339)),
				},
			},
			expected: &clientauthentication.ExecCredential{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "client.authentication.k8s.io/v1",
					Kind:       "ExecCredential",
				},
				Status: &clientauthentication.ExecCredentialStatus{
					ExpirationTimestamp: &metav1.Time{Time: t0.Add(24 * time.Hour).Local().Truncate(time.Second)},
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

func cleanAllCache(t *testing.T) {
	t.Helper()

	credentials.Range(func(key, value any) bool {
		credentials.Delete(key)
		return true
	})
}
