package service

import (
	"strings"
	"testing"

	"simplehpc/backend/internal/integrations/slurm"
)

func TestNormalizeJobQuery(t *testing.T) {
	tests := []struct {
		name       string
		page       int
		pageSize   int
		status     string
		wantPage   int
		wantSize   int
		wantOffset int
		wantStatus string
	}{
		{name: "defaults", wantPage: 1, wantSize: 15, wantOffset: 0},
		{name: "middle page", page: 18, pageSize: 30, status: "运行中", wantPage: 18, wantSize: 30, wantOffset: 510, wantStatus: "RUNNING"},
		{name: "caps page size", page: 2, pageSize: 999, status: "完成", wantPage: 2, wantSize: 100, wantOffset: 100, wantStatus: "COMPLETED"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeJobQuery(JobQuery{Page: tt.page, PageSize: tt.pageSize, Status: tt.status})
			if got.Page != tt.wantPage || got.PageSize != tt.wantSize || got.Offset != tt.wantOffset || got.Status != tt.wantStatus {
				t.Fatalf("normalizeJobQuery() = %#v", got)
			}
		})
	}
}

func TestScopeJobQueryLimitsRegularUserToOwnJobs(t *testing.T) {
	query := ScopeJobQuery(AuthUser{Username: "user001", Type: "user"}, JobQuery{
		Username:  "root",
		Group:     "admins",
		Partition: "debug",
	})

	if query.Username != "user001" {
		t.Fatalf("Username = %q, want current user", query.Username)
	}
	if query.Group != "" {
		t.Fatalf("Group = %q, want regular-user group filter removed", query.Group)
	}
	if query.Partition != "debug" {
		t.Fatalf("Partition = %q, want partition filter preserved within own jobs", query.Partition)
	}
}

func TestScopeJobQueryKeepsAdministratorFilters(t *testing.T) {
	want := JobQuery{Username: "user001", Group: "cs_group", Partition: "debug"}
	got := ScopeJobQuery(AuthUser{Username: "testadmin", Type: "admin"}, want)

	if got.Username != want.Username || got.Group != want.Group || got.Partition != want.Partition {
		t.Fatalf("ScopeJobQuery() = %#v, want administrator filters unchanged", got)
	}
}

func TestBuildJobWhereIncludesAdministratorFilters(t *testing.T) {
	where, args := buildJobWhere(JobQuery{
		Username:  "user001",
		Group:     "cs_group",
		Partition: "debug",
	})

	if !strings.Contains(where, "user_name = $1") {
		t.Fatalf("where does not filter exact user: %s", where)
	}
	if !strings.Contains(where, "partition = $2") {
		t.Fatalf("where does not filter exact partition: %s", where)
	}
	if !strings.Contains(where, "platform_users") || !strings.Contains(where, "teams") {
		t.Fatalf("where does not resolve platform user groups: %s", where)
	}
	if len(args) != 3 || args[0] != "user001" || args[1] != "debug" || args[2] != "cs_group" {
		t.Fatalf("args = %#v, want exact user, partition and group", args)
	}
}

func TestBuildJobWhereAppliesRBACOrganizationScopes(t *testing.T) {
	where, args := buildJobWhere(JobQuery{UnitIDs: []string{"11", "12"}, TeamIDs: []string{"21"}})
	if !strings.Contains(where, "unit_id::text = ANY") || !strings.Contains(where, "team_id::text = ANY") {
		t.Fatalf("organization filters missing: %s", where)
	}
	if len(args) != 2 {
		t.Fatalf("args = %#v", args)
	}
}

func TestBuildJobWhereCanDenyAll(t *testing.T) {
	where, args := buildJobWhere(JobQuery{DenyAll: true})
	if !strings.Contains(where, "1=0") || len(args) != 0 {
		t.Fatalf("deny-all query = %s %#v", where, args)
	}
}

func TestStaleQueueSnapshotDeleteStatement(t *testing.T) {
	query, args := staleQueueSnapshotDeleteStatement([]slurm.Job{
		{ID: "321"},
		{ID: "322_7"},
		{ID: "321"},
	})

	if !strings.Contains(query, "source = 'squeue'") {
		t.Fatalf("query does not limit cleanup to squeue snapshots: %s", query)
	}
	if !strings.Contains(query, "job_id NOT IN ($1,$2)") {
		t.Fatalf("query does not preserve current queue jobs: %s", query)
	}
	if len(args) != 2 || args[0] != "321" || args[1] != "322_7" {
		t.Fatalf("args = %#v, want current unique job ids", args)
	}
}

func TestStaleQueueSnapshotDeleteStatementWithEmptyQueue(t *testing.T) {
	query, args := staleQueueSnapshotDeleteStatement(nil)

	if strings.Contains(query, "job_id NOT IN") {
		t.Fatalf("empty queue cleanup must not add a NOT IN clause: %s", query)
	}
	if len(args) != 0 {
		t.Fatalf("args = %#v, want none", args)
	}
}
