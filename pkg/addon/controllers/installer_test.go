package controllers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
)

// The generic apiserver's default admission plugins start informers on these
// resources. If the gateway cannot list/watch one of them the informer never
// syncs and every write request fails with "not yet ready to handle request".
func TestAPFClusterRoleCoversAdmissionPluginInformers(t *testing.T) {
	role := newAPFClusterRole(&addonv1alpha1.ClusterManagementAddOn{})
	for _, resource := range []string{
		"mutatingwebhookconfigurations",
		"validatingwebhookconfigurations",
		"validatingadmissionpolicies",
		"validatingadmissionpolicybindings",
		"mutatingadmissionpolicies",
		"mutatingadmissionpolicybindings",
	} {
		for _, verb := range []string{"get", "list", "watch"} {
			assert.Truef(t, allows(role.Rules, "admissionregistration.k8s.io", resource, verb),
				"cluster role does not allow %s on %s", verb, resource)
		}
	}
}

func allows(rules []rbacv1.PolicyRule, group, resource, verb string) bool {
	for _, r := range rules {
		if contains(r.APIGroups, group) && contains(r.Resources, resource) && contains(r.Verbs, verb) {
			return true
		}
	}
	return false
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
