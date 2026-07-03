package service

import "testing"

func TestScopeJobQueryWithPermissionContext(t *testing.T) {
	tests := []struct {
		name      string
		scopes    []DataScope
		unitIDs   []string
		teamIDs   []string
		wantUser  string
		wantUnits []string
		wantTeams []string
	}{
		{name: "self", scopes: []DataScope{ScopeSelf}, wantUser: "user001"},
		{name: "team", scopes: []DataScope{ScopeTeam}, teamIDs: []string{"t1"}, wantTeams: []string{"t1"}},
		{name: "unit", scopes: []DataScope{ScopeUnit}, unitIDs: []string{"u1"}, wantUnits: []string{"u1"}},
		{name: "global", scopes: []DataScope{ScopeGlobal}, wantUser: "attacker"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := PermissionContext{
				Username: "user001", DataScopes: map[string]map[DataScope]struct{}{"jobs": {}},
				UnitIDs: tt.unitIDs, TeamIDs: tt.teamIDs,
			}
			for _, scope := range tt.scopes {
				ctx.DataScopes["jobs"][scope] = struct{}{}
			}
			got := ScopeJobQueryByPermission(ctx, JobQuery{Username: "attacker", Group: "other"})
			if got.Username != tt.wantUser || !sameStrings(got.UnitIDs, tt.wantUnits) || !sameStrings(got.TeamIDs, tt.wantTeams) {
				t.Fatalf("scoped query = %#v", got)
			}
		})
	}
}

func TestNoneScopeProducesImpossibleJobQuery(t *testing.T) {
	ctx := PermissionContext{Username: "user001", DataScopes: map[string]map[DataScope]struct{}{}}
	if got := ScopeJobQueryByPermission(ctx, JobQuery{}); !got.DenyAll {
		t.Fatalf("none scope did not deny query: %#v", got)
	}
}

func TestPermissionContextAllowsResourceIdentity(t *testing.T) {
	authz := PermissionContext{
		Username: "manager", UnitIDs: []string{"u1"}, TeamIDs: []string{"t1"},
		DataScopes: map[string]map[DataScope]struct{}{
			"jobs": {ScopeSelf: {}, ScopeTeam: {}},
		},
	}
	if !authz.Allows("jobs", ResourceIdentity{Owner: "manager"}) {
		t.Fatal("self-owned resource denied")
	}
	if !authz.Allows("jobs", ResourceIdentity{Owner: "other", TeamID: "t1"}) {
		t.Fatal("team resource denied")
	}
	if authz.Allows("jobs", ResourceIdentity{Owner: "other", TeamID: "t2", UnitID: "u2"}) {
		t.Fatal("out-of-scope resource allowed")
	}
}

func TestGrantedScopeRequiresExplicitGrant(t *testing.T) {
	authz := PermissionContext{
		Username:   "user001",
		DataScopes: map[string]map[DataScope]struct{}{"job_templates": {ScopeGranted: {}}},
	}
	if authz.Allows("job_templates", ResourceIdentity{Owner: "other"}) {
		t.Fatal("ungranted resource allowed")
	}
	if !authz.Allows("job_templates", ResourceIdentity{Owner: "other", Granted: true}) {
		t.Fatal("explicitly granted resource denied")
	}
}

func sameStrings(a, b []string) bool {
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
