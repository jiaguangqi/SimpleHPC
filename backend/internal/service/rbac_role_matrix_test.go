package service

import "testing"

func TestCompleteRoleDecisionMatrix(t *testing.T) {
	tests := []struct {
		name        string
		grants      []RoleGrant
		permission  string
		wantAllowed bool
		wantScope   DataScope
		wantFiles   int
	}{
		{
			name: "ordinary user", permission: "api.jobs.list", wantAllowed: true,
			wantScope: ScopeSelf, wantFiles: 1,
			grants: []RoleGrant{{Code: "user", Status: RoleActive,
				Permissions:  []string{"api.jobs.list"},
				DataScopes:   []DataScopeGrant{{Resource: "jobs", Scope: ScopeSelf, Access: AccessManage}},
				FilePolicies: []FilePolicyGrant{{StorageRoot: "/data/home", SubjectScope: "self", Access: AccessManage}}}},
		},
		{
			name: "cluster admin", permission: "api.roles.delete", wantAllowed: true,
			wantScope: ScopeGlobal,
			grants:    []RoleGrant{{Code: ClusterAdminRole, Status: RoleActive}},
		},
		{
			name: "config admin cannot read user files", permission: "api.config.slurm.list", wantAllowed: true,
			wantScope: ScopeNone, wantFiles: 0,
			grants: []RoleGrant{{Code: "config_admin", Status: RoleActive,
				Permissions: []string{"api.config.slurm.list"}}},
		},
		{
			name: "unit admin", permission: "api.jobs.list", wantAllowed: true,
			wantScope: ScopeUnit, wantFiles: 1,
			grants: []RoleGrant{{Code: "unit_admin", Status: RoleActive,
				Permissions:  []string{"api.jobs.list"},
				DataScopes:   []DataScopeGrant{{Resource: "jobs", Scope: ScopeUnit, Access: AccessManage}},
				FilePolicies: []FilePolicyGrant{{StorageRoot: "/data/share", SubjectScope: "unit_shared", Access: AccessManage}}}},
		},
		{
			name: "team admin", permission: "api.jobs.list", wantAllowed: true,
			wantScope: ScopeTeam, wantFiles: 1,
			grants: []RoleGrant{{Code: "team_admin", Status: RoleActive,
				Permissions:  []string{"api.jobs.list"},
				DataScopes:   []DataScopeGrant{{Resource: "jobs", Scope: ScopeTeam, Access: AccessManage}},
				FilePolicies: []FilePolicyGrant{{StorageRoot: "/data/share", SubjectScope: "team_shared", Access: AccessManage}}}},
		},
		{
			name: "custom observer", permission: "api.jobs.list", wantAllowed: true,
			wantScope: ScopeGranted,
			grants: []RoleGrant{{Code: "job_observer", Status: RoleActive,
				Permissions: []string{"api.jobs.list"},
				DataScopes:  []DataScopeGrant{{Resource: "jobs", Scope: ScopeGranted, Access: AccessView}}}},
		},
		{
			name: "multi role union", permission: "api.jobs.cancel", wantAllowed: true,
			wantScope: ScopeTeam, wantFiles: 1,
			grants: []RoleGrant{
				{Code: "user", Status: RoleActive, Permissions: []string{"api.jobs.list"},
					DataScopes:   []DataScopeGrant{{Resource: "jobs", Scope: ScopeSelf, Access: AccessManage}},
					FilePolicies: []FilePolicyGrant{{StorageRoot: "/data/home", SubjectScope: "self", Access: AccessManage}}},
				{Code: "reviewer", Status: RoleActive, Permissions: []string{"api.jobs.cancel"},
					DataScopes: []DataScopeGrant{{Resource: "jobs", Scope: ScopeTeam, Access: AccessView}}},
			},
		},
		{
			name: "disabled role", permission: "api.roles.list", wantAllowed: false,
			wantScope: ScopeNone,
			grants: []RoleGrant{{Code: "disabled_admin", Status: RoleDisabled,
				Permissions: []string{"api.roles.list"},
				DataScopes:  []DataScopeGrant{{Resource: "jobs", Scope: ScopeGlobal, Access: AccessManage}}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := MergeRoleGrants("matrix-user", "ldap", tt.grants)
			if got := ctx.Has(tt.permission); got != tt.wantAllowed {
				t.Fatalf("permission allowed=%v want=%v: %#v", got, tt.wantAllowed, ctx.Permissions)
			}
			if got := ctx.HighestScope("jobs"); got != tt.wantScope {
				t.Fatalf("jobs scope=%q want=%q", got, tt.wantScope)
			}
			if got := len(ctx.FilePolicies); got != tt.wantFiles {
				t.Fatalf("file policies=%d want=%d: %#v", got, tt.wantFiles, ctx.FilePolicies)
			}
		})
	}
}
