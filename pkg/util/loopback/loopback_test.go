package loopback

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/client-go/rest"
)

// unconditionalOnly implements only Authorize, matching the
// authorizer.UnconditionalAuthorizer that GenericAPIServer exposes.
type unconditionalOnly struct{ reason string }

func (u unconditionalOnly) Authorize(context.Context, authorizer.Attributes) (authorizer.Decision, string, error) {
	return authorizer.DecisionAllow, u.reason, nil
}

func TestAuthorizer(t *testing.T) {
	SetAuthorizer(unconditionalOnly{reason: "first"})
	decision, reason, err := GetAuthorizer().Authorize(context.TODO(), nil)
	assert.NoError(t, err)
	assert.Equal(t, authorizer.DecisionAllow, decision)
	assert.Equal(t, "first", reason)

	// SetAuthorizer is once-only: a later call must not replace the value
	// wired during bootstrap.
	second := authorizer.AuthorizerFunc(func(context.Context, authorizer.Attributes) (authorizer.Decision, string, error) {
		return authorizer.DecisionDeny, "second", nil
	})
	SetAuthorizer(second)
	_, reason, _ = GetAuthorizer().Authorize(context.TODO(), nil)
	assert.Equal(t, "first", reason)
}

func TestLoopbackMasterClientConfig(t *testing.T) {
	first := &rest.Config{Host: "https://first"}
	SetLoopbackMasterClientConfig(first)
	assert.Equal(t, first, GetLoopbackMasterClientConfig())

	// SetLoopbackMasterClientConfig is once-only: a later call must not
	// replace the value wired during bootstrap.
	SetLoopbackMasterClientConfig(&rest.Config{Host: "https://second"})
	assert.Equal(t, first, GetLoopbackMasterClientConfig())
}
