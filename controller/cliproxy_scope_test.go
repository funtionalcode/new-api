package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

func TestCliproxyBindingQueryScopeLimitsRegularUsers(t *testing.T) {
	query := model.CliproxyAuthFileBindingQuery{
		Username: "alice",
	}

	scoped := cliproxyBindingQueryForRole(query, 42, common.RoleCommonUser)

	if scoped.UserId != 42 {
		t.Fatalf("UserId = %d, want 42", scoped.UserId)
	}
	if scoped.Username != "" {
		t.Fatalf("Username = %q, want empty for regular users", scoped.Username)
	}
}

func TestCliproxyBindingQueryScopeKeepsAdminFilters(t *testing.T) {
	query := model.CliproxyAuthFileBindingQuery{
		Username: "alice",
	}

	scoped := cliproxyBindingQueryForRole(query, 42, common.RoleAdminUser)

	if scoped.UserId != 0 {
		t.Fatalf("UserId = %d, want 0 for admins", scoped.UserId)
	}
	if scoped.Username != "alice" {
		t.Fatalf("Username = %q, want alice", scoped.Username)
	}
}
