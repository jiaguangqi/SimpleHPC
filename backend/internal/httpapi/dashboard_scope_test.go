package httpapi

import (
	"testing"

	"simplehpc/backend/internal/service"
)

func TestDashboardJobQueryUsesRBACDataScopeInShadow(t *testing.T) {
	tests := []struct {
		name     string
		authz    service.PermissionContext
		username string
		unitIDs  []string
		teamIDs  []string
		global   bool
	}{
		{
			name: "ordinary user sees self",
			authz: service.PermissionContext{
				Username:   "user001",
				DataScopes: map[string]map[service.DataScope]struct{}{"jobs": {service.ScopeSelf: {}}},
			},
			username: "user001",
		},
		{
			name: "team administrator sees team",
			authz: service.PermissionContext{
				Username: "team-admin", TeamIDs: []string{"15"},
				DataScopes: map[string]map[service.DataScope]struct{}{"jobs": {service.ScopeTeam: {}}},
			},
			teamIDs: []string{"15"},
		},
		{
			name: "unit administrator sees unit",
			authz: service.PermissionContext{
				Username: "unit-admin", UnitIDs: []string{"1126"},
				DataScopes: map[string]map[service.DataScope]struct{}{"jobs": {service.ScopeUnit: {}}},
			},
			unitIDs: []string{"1126"},
		},
		{
			name: "cluster administrator sees global",
			authz: service.PermissionContext{
				Username: "cluster-admin", IsClusterAdmin: true,
			},
			global: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dashboardJobQuery(tt.authz)
			if got.Username != tt.username {
				t.Fatalf("username=%q want=%q", got.Username, tt.username)
			}
			if !equalStrings(got.UnitIDs, tt.unitIDs) {
				t.Fatalf("unitIDs=%v want=%v", got.UnitIDs, tt.unitIDs)
			}
			if !equalStrings(got.TeamIDs, tt.teamIDs) {
				t.Fatalf("teamIDs=%v want=%v", got.TeamIDs, tt.teamIDs)
			}
			if tt.global && (got.Username != "" || len(got.UnitIDs) != 0 || len(got.TeamIDs) != 0 || got.DenyAll) {
				t.Fatalf("cluster administrator query unexpectedly scoped: %#v", got)
			}
		})
	}
}

func TestDashboardQueueTrendRouteUsesDashboardPermission(t *testing.T) {
	key, public := routePermission("GET", "/api/v1/dashboard/queue-job-trends")
	if public || key != "api.dashboard.list" {
		t.Fatalf("queue trend permission = %q public=%v, want api.dashboard.list", key, public)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
