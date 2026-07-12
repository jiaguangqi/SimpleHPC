package service

import (
	"testing"

	slurmintegration "simplehpc/backend/internal/integrations/slurm"
)

func TestStaleProjectAssociations(t *testing.T) {
	members := []ProjectMember{
		{Username: "owner", Status: "active"},
		{Username: "disabled", Status: "disabled"},
	}
	associations := []slurmintegration.Association{
		{User: "owner", Account: "demo-cfd"},
		{User: "disabled", Account: "demo-cfd", DefaultAccount: "simplehpc"},
		{User: "removed", Account: "demo-cfd", DefaultAccount: "demo-cfd"},
		{User: "removed", Account: "demo-cfd", Partition: "gpu", DefaultAccount: "demo-cfd"},
		{User: "other", Account: "other-project"},
	}
	stale := staleProjectAssociations("demo-cfd", members, associations)
	if len(stale) != 2 || stale[0].User != "disabled" || stale[1].User != "removed" {
		t.Fatalf("stale associations = %#v", stale)
	}
}
