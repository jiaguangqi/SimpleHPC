package service

import (
	"testing"
	"time"

	"simplehpc/backend/internal/integrations/slurm"
)

func TestAcknowledgeAlertChangesActiveAlert(t *testing.T) {
	now := time.Date(2026, 6, 30, 10, 0, 0, 0, time.UTC)
	alert := DashboardAlert{ID: 7, Status: "active"}
	got := acknowledgeAlert(alert, "testadmin", now)

	if got.Status != "acknowledged" || got.AcknowledgedBy != "testadmin" || got.AcknowledgedAt != now.Format(time.RFC3339) {
		t.Fatalf("unexpected alert: %#v", got)
	}
}

func TestNodeAlertCandidatesOnlyIncludesUnhealthyNodes(t *testing.T) {
	nodes := []slurm.Node{
		{Name: "cae", State: "idle"},
		{Name: "gpu-01", State: "drain"},
		{Name: "cpu-02", State: "down*"},
	}
	got := nodeAlertCandidates(nodes)

	if len(got) != 2 {
		t.Fatalf("len = %d, want 2: %#v", len(got), got)
	}
	if got[0].Source != "slurm-node:gpu-01" || got[1].Source != "slurm-node:cpu-02" {
		t.Fatalf("unexpected sources: %#v", got)
	}
}
