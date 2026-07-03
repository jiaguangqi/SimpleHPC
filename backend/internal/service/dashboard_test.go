package service

import (
	"testing"

	"simplehpc/backend/internal/integrations/slurm"
)

func TestDashboardTrendRangeConfig(t *testing.T) {
	tests := []struct {
		input      string
		wantRange  string
		wantWindow string
		wantBucket string
	}{
		{input: "24h", wantRange: "24h", wantWindow: "24 hours", wantBucket: "15 minutes"},
		{input: "7d", wantRange: "7d", wantWindow: "7 days", wantBucket: "1 hour"},
		{input: "30d", wantRange: "30d", wantWindow: "30 days", wantBucket: "6 hours"},
		{input: "90d", wantRange: "90d", wantWindow: "90 days", wantBucket: "1 day"},
		{input: "1y", wantRange: "1y", wantWindow: "1 year", wantBucket: "7 days"},
		{input: "unexpected", wantRange: "7d", wantWindow: "7 days", wantBucket: "1 hour"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := dashboardTrendRangeConfig(tt.input)
			if got.Name != tt.wantRange || got.Window != tt.wantWindow || got.Bucket != tt.wantBucket {
				t.Fatalf("dashboardTrendRangeConfig(%q) = %#v", tt.input, got)
			}
		})
	}
}

func TestChooseVisibleQueuePrefersRequestedThenDebug(t *testing.T) {
	queues := []string{"gpu", "debug", "normal"}
	if got := chooseVisibleQueue("gpu", queues); got != "gpu" {
		t.Fatalf("requested visible queue = %q, want gpu", got)
	}
	if got := chooseVisibleQueue("hidden", queues); got != "debug" {
		t.Fatalf("hidden requested queue fallback = %q, want debug", got)
	}
	if got := chooseVisibleQueue("", []string{"normal", "short"}); got != "normal" {
		t.Fatalf("default queue = %q, want first visible queue", got)
	}
	if got := chooseVisibleQueue("debug", nil); got != "" {
		t.Fatalf("empty queues = %q, want empty", got)
	}
}

func TestSummarizeCurrentJobs(t *testing.T) {
	jobs := []slurm.Job{
		{ID: "1", State: "RUNNING", CPUs: "8", GPUs: "2"},
		{ID: "2", State: "R", CPUs: "4", GPUs: "0"},
		{ID: "3", State: "PENDING", CPUs: "16", GPUs: "4"},
		{ID: "4", State: "COMPLETED", CPUs: "2", GPUs: "0"},
	}

	got := summarizeCurrentJobs(jobs)
	if got.Running != 2 || got.Pending != 1 || got.AllocatedCPUs != 12 || got.AllocatedGPUs != 2 {
		t.Fatalf("summarizeCurrentJobs() = %#v", got)
	}
}

func TestSummarizeNodesWithoutGPU(t *testing.T) {
	nodes := []slurm.Node{
		{Name: "node01", State: "idle", CPUTotal: "20", CPUAllocated: "0", GRES: "(null)"},
	}

	got := summarizeNodes(nodes)
	if got.TotalNodes != 1 || got.TotalCPUs != 20 || got.AllocatedCPUs != 0 || got.IdleNodes != 1 {
		t.Fatalf("summarizeNodes() = %#v", got)
	}
	if got.TotalGPUs != 0 || got.GPUUsage != nil || got.UnavailableGPU != "当前 Slurm 集群未配置 GPU 资源" {
		t.Fatalf("unexpected GPU summary: %#v", got)
	}
}

func TestUniqueAuthUsers(t *testing.T) {
	values := [][]byte{
		[]byte(`{"username":"alice","type":"ldap"}`),
		[]byte(`{"username":"alice","type":"ldap"}`),
		[]byte(`{"username":"admin","type":"admin"}`),
		[]byte(`not-json`),
	}
	if got := uniqueAuthUsers(values); got != 2 {
		t.Fatalf("uniqueAuthUsers() = %d, want 2", got)
	}
}
