package cert

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCopySecret(t *testing.T) {
	cases := []struct {
		name            string
		sourceNamespace string
		sourceName      string
		targetNamespace string
		targetName      string
		source          *corev1.Secret
		existing        *corev1.Secret
		expected        *corev1.Secret
		errAssert       func(err error) bool
	}{
		{
			name:            "target should be created",
			sourceNamespace: "ns1",
			sourceName:      "s1",
			targetNamespace: "ns2",
			targetName:      "s2",
			source: newSecret("ns1", "s1", map[string][]byte{
				"k1": []byte("v1"),
			}),
			existing: nil,
			expected: newSecret("ns2", "s2", map[string][]byte{
				"k1": []byte("v1"),
			}),
		},
		{
			name:            "diff should be reconciled",
			sourceNamespace: "ns1",
			sourceName:      "s1",
			targetNamespace: "ns2",
			targetName:      "s2",
			source: newSecret("ns1", "s1", map[string][]byte{
				"k1": []byte("v1"),
			}),
			existing: newSecret("ns2", "s2", map[string][]byte{
				"k1": []byte("v2"),
			}),
			expected: newSecret("ns2", "s2", map[string][]byte{
				"k1": []byte("v1"),
			}),
		},
		{
			name:            "extra content should be kept",
			sourceNamespace: "ns1",
			sourceName:      "s1",
			targetNamespace: "ns2",
			targetName:      "s2",
			source: newSecret("ns1", "s1", map[string][]byte{
				"k1": []byte("v1"),
			}),
			existing: newSecret("ns2", "s2", map[string][]byte{
				"k1": []byte("v1"),
				"k2": []byte("v2"),
			}),
			expected: newSecret("ns2", "s2", map[string][]byte{
				"k1": []byte("v1"),
				"k2": []byte("v2"),
			}),
		},
		{
			name:            "no source should error",
			sourceNamespace: "ns1",
			sourceName:      "s1",
			targetNamespace: "ns2",
			targetName:      "s2",
			errAssert: func(err error) bool {
				return strings.HasPrefix(err.Error(), "failed getting source secret")
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			objs := []runtime.Object{}
			if c.source != nil {
				objs = append(objs, c.source)
			}
			if c.existing != nil {
				objs = append(objs, c.existing)
			}
			client := fake.NewSimpleClientset(objs...)
			err := CopySecret(client, c.sourceNamespace, c.sourceName, c.targetNamespace, c.targetName)
			if c.errAssert != nil {
				assert.True(t, c.errAssert(err))
				return
			}
			assert.NoError(t, err)
			actual, err := client.CoreV1().Secrets(c.targetNamespace).
				Get(context.TODO(), c.targetName, metav1.GetOptions{})
			assert.NoError(t, err)
			assert.Equal(t, c.expected, actual)
		})
	}
}

func newSecret(namespace, name string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Data: data,
	}
}
