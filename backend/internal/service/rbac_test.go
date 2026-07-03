package service

import (
	"strings"
	"testing"
	"time"
)

func TestMergeRoleGrantsUsesUnionAndStrongestLevels(t *testing.T) {
	ctx := MergeRoleGrants("user001", "ldap", []RoleGrant{
		{
			Code: "user", Status: RoleActive,
			Permissions: []string{"menu.dashboard.view", "action.jobs.view"},
			DataScopes: []DataScopeGrant{
				{Resource: "jobs", Scope: ScopeSelf, Access: AccessView},
				{Resource: "job_templates", Scope: ScopeGranted, Access: AccessView},
			},
		},
		{
			Code: "job_reviewer", Status: RoleActive,
			Permissions: []string{"action.jobs.view", "action.jobs.cancel"},
			DataScopes: []DataScopeGrant{
				{Resource: "jobs", Scope: ScopeTeam, Access: AccessManage},
				{Resource: "job_templates", Scope: ScopeSelf, Access: AccessView},
			},
		},
	})

	for _, key := range []string{"menu.dashboard.view", "action.jobs.view", "action.jobs.cancel"} {
		if !ctx.Has(key) {
			t.Fatalf("merged permissions missing %q", key)
		}
	}
	if got := ctx.AccessLevel("jobs"); got != AccessManage {
		t.Fatalf("jobs access = %q, want manage", got)
	}
	if got := ctx.HighestScope("jobs"); got != ScopeTeam {
		t.Fatalf("jobs scope = %q, want team", got)
	}
	if !ctx.HasScope("job_templates", ScopeSelf) || !ctx.HasScope("job_templates", ScopeGranted) {
		t.Fatalf("self and granted must both remain: %#v", ctx.DataScopes["job_templates"])
	}
}

func TestMergeRoleGrantsIgnoresDisabledRoles(t *testing.T) {
	ctx := MergeRoleGrants("user001", "ldap", []RoleGrant{
		{Code: "user", Status: RoleActive, Permissions: []string{"menu.dashboard.view"}},
		{Code: "disabled_admin", Status: RoleDisabled, Permissions: []string{"menu.roles.view"}},
	})
	if ctx.Has("menu.roles.view") {
		t.Fatal("disabled role granted permission")
	}
}

func TestClusterAdminGetsEmergencyFallback(t *testing.T) {
	ctx := MergeRoleGrants("admin", "admin", []RoleGrant{
		{Code: ClusterAdminRole, Status: RoleActive},
	})
	if !ctx.IsClusterAdmin || !ctx.Has("*") {
		t.Fatalf("cluster_admin fallback missing: %#v", ctx)
	}
	if got := ctx.HighestScope("jobs"); got != ScopeGlobal {
		t.Fatalf("cluster_admin scope = %q, want global", got)
	}
}

func TestFilePoliciesDoNotExpandFromOrdinaryDataScope(t *testing.T) {
	ctx := MergeRoleGrants("user001", "ldap", []RoleGrant{{
		Code: "data_observer", Status: RoleActive,
		DataScopes: []DataScopeGrant{{Resource: "storage_files", Scope: ScopeGlobal, Access: AccessView}},
	}})
	if len(ctx.FilePolicies) != 0 {
		t.Fatalf("ordinary data scope expanded file policies: %#v", ctx.FilePolicies)
	}
}

func TestOrdinaryUserGrantDoesNotIncludeAdministratorAPI(t *testing.T) {
	ctx := MergeRoleGrants("user001", "ldap", []RoleGrant{{
		Code: "user", Status: RoleActive,
		Permissions: []string{"api.jobs.list", "api.storage.files.list"},
	}})
	if ctx.Has("api.users.list") || ctx.Has("api.roles.list") {
		t.Fatal("ordinary user inherited administrator API")
	}
}

func TestIntermediateAdminRoleCatalogAccessIsReadOnly(t *testing.T) {
	readOnly := []string{
		"menu.account.roles.view",
		"route.account.roles.view",
		"api.roles.list",
		"action.roles.view",
	}
	mutations := []string{
		"api.roles.create",
		"api.roles.update",
		"api.roles.delete",
		"action.roles.create",
		"action.roles.edit",
		"action.roles.delete",
		"action.roles.copy",
		"action.roles.assign",
		"action.roles.permissions.manage",
	}
	for _, role := range []string{"config_admin", "unit_admin", "team_admin"} {
		t.Run(role, func(t *testing.T) {
			ctx := MergeRoleGrants(role+"-user", "admin", []RoleGrant{{
				Code: role, Status: RoleActive, Permissions: readOnly,
			}})
			for _, key := range readOnly {
				if !ctx.Has(key) {
					t.Fatalf("%s missing readonly permission %q", role, key)
				}
			}
			for _, key := range mutations {
				if ctx.Has(key) {
					t.Fatalf("%s unexpectedly has mutating permission %q", role, key)
				}
			}
		})
	}
}

func TestValidateRoleMutationProtectsBuiltins(t *testing.T) {
	tests := []struct {
		name string
		role RoleDefinition
		op   RoleMutation
	}{
		{"disable cluster admin", RoleDefinition{Code: ClusterAdminRole, IsBuiltin: true}, MutationDisable},
		{"delete builtin", RoleDefinition{Code: "user", IsBuiltin: true, AllowDelete: false}, MutationDelete},
		{"clear protected permissions", RoleDefinition{Code: ClusterAdminRole, IsBuiltin: true, AllowPermissionEdit: false}, MutationReplacePermissions},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateRoleMutation(tt.role, tt.op); err == nil {
				t.Fatal("protected role mutation was accepted")
			}
		})
	}
}

func TestValidateLastClusterAdminBinding(t *testing.T) {
	if err := ValidateClusterAdminBindingRemoval(1); err == nil || !strings.Contains(err.Error(), "最后") {
		t.Fatalf("last binding removal error = %v", err)
	}
	if err := ValidateClusterAdminBindingRemoval(2); err != nil {
		t.Fatalf("removing one of multiple bindings failed: %v", err)
	}
}

func TestPermissionVersionChangesWithRoleOrBinding(t *testing.T) {
	first := permissionVersion([]permissionVersionPart{{
		RoleCode: "user", RoleVersion: 1, BindingUpdatedAt: time.Unix(100, 0),
	}})
	roleChanged := permissionVersion([]permissionVersionPart{{
		RoleCode: "user", RoleVersion: 2, BindingUpdatedAt: time.Unix(100, 0),
	}})
	bindingChanged := permissionVersion([]permissionVersionPart{{
		RoleCode: "user", RoleVersion: 1, BindingUpdatedAt: time.Unix(101, 0),
	}})
	if first == roleChanged || first == bindingChanged {
		t.Fatal("permission version did not change")
	}
}

func TestPermissionCacheKeySeparatesAccountTypes(t *testing.T) {
	if permissionCacheKey("admin", "same") == permissionCacheKey("ldap", "same") {
		t.Fatal("admin and ldap cache keys collided")
	}
}

func TestIntermediateAdministratorScopesDoNotCrossOrganizations(t *testing.T) {
	unitAdmin := MergeRoleGrants("unit_admin_1", "admin", []RoleGrant{{
		Code: "unit_admin", Status: RoleActive,
		DataScopes: []DataScopeGrant{{Resource: "jobs", Scope: ScopeUnit, Access: AccessManage}},
	}})
	unitAdmin.UnitIDs = []string{"unit-a"}
	if !unitAdmin.Allows("jobs", ResourceIdentity{UnitID: "unit-a"}) {
		t.Fatal("unit_admin cannot access own unit")
	}
	if unitAdmin.Allows("jobs", ResourceIdentity{UnitID: "unit-b"}) {
		t.Fatal("unit_admin crossed unit boundary")
	}

	teamAdmin := MergeRoleGrants("team_admin_1", "admin", []RoleGrant{{
		Code: "team_admin", Status: RoleActive,
		DataScopes: []DataScopeGrant{{Resource: "jobs", Scope: ScopeTeam, Access: AccessManage}},
	}})
	teamAdmin.TeamIDs = []string{"team-a"}
	if !teamAdmin.Allows("jobs", ResourceIdentity{TeamID: "team-a"}) {
		t.Fatal("team_admin cannot access own team")
	}
	if teamAdmin.Allows("jobs", ResourceIdentity{TeamID: "team-b"}) {
		t.Fatal("team_admin crossed team boundary")
	}
}

func TestMultiRoleDataScopeStillDoesNotCreateFilePolicy(t *testing.T) {
	ctx := MergeRoleGrants("multi", "ldap", []RoleGrant{
		{Code: "user", Status: RoleActive, FilePolicies: []FilePolicyGrant{
			{StorageRoot: "/data/home", SubjectScope: "self", Access: AccessManage},
		}},
		{Code: "global_observer", Status: RoleActive,
			DataScopes: []DataScopeGrant{{Resource: "storage_files", Scope: ScopeGlobal, Access: AccessView}}},
	})
	if len(ctx.FilePolicies) != 1 || ctx.FilePolicies[0].SubjectScope != "self" {
		t.Fatalf("data-scope role expanded file policy: %#v", ctx.FilePolicies)
	}
}
