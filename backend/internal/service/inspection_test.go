package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestInspectionSummaryIsWarningWhenAnyCheckFails(t *testing.T) {
	checks := []InspectionCheck{
		{Name: "PostgreSQL", Status: "ok"},
		{Name: "Slurm", Status: "error", Message: "slurmctld unavailable"},
	}
	if got := summarizeInspection(checks); got != "warning" {
		t.Fatalf("got %q, want warning", got)
	}
}

func TestRunInspectionCommandCapturesEvidence(t *testing.T) {
	result := runInspectionCommand(context.Background(), InspectionCommand{
		Name:    "shell evidence",
		Command: "/bin/sh",
		Args:    []string{"-c", "echo standard; echo problem >&2; exit 3"},
	})
	if result.ExitCode != 3 || result.Stdout != "standard" || result.Stderr != "problem" {
		t.Fatalf("unexpected command evidence: %#v", result)
	}
	if result.Command != "/bin/sh -c echo standard; echo problem >&2; exit 3" || result.DurationMS < 0 {
		t.Fatalf("missing trace metadata: %#v", result)
	}
}

func TestMissingInspectionCommandIsSkipped(t *testing.T) {
	result := runInspectionCommand(context.Background(), InspectionCommand{
		Name:             "GPU",
		Command:          "/definitely/missing/nvidia-smi",
		SkipWhenMissing:  true,
		UnavailableLabel: "未安装 NVIDIA 工具",
	})
	if result.Status != "skipped" || !strings.Contains(result.Message, "未安装 NVIDIA 工具") {
		t.Fatalf("unexpected missing-command result: %#v", result)
	}
}

func TestInspectionReportUsesA3AndRealSummary(t *testing.T) {
	run := InspectionRun{
		RunID: "20260630-142805", Status: "warning", ClusterName: "simplehpc-dev",
		Checks:  []InspectionCheck{{Name: "Slurm 节点", Category: "调度系统", Status: "ok", Message: "1 个节点"}},
		Summary: map[string]any{"nodes": 1, "cpuCores": 20, "memoryGB": 29.3, "gpuCount": 0},
	}
	report := RenderInspectionHTML(run)
	for _, expected := range []string{"@page{size:A3 portrait", "simplehpc-dev", "20", "Slurm 节点", "导出 A3 PDF"} {
		if !strings.Contains(report, expected) {
			t.Fatalf("report missing %q", expected)
		}
	}
}

func TestStorageSummaryDeduplicatesSharedFilesystem(t *testing.T) {
	checks := []InspectionCheck{
		{Name: "存储容量 /data/home", Stdout: "Filesystem 1-blocks Used Available Capacity Mounted on\n/dev/sdb 214643507200 1 1 1% /data"},
		{Name: "存储容量 /data/share", Stdout: "Filesystem 1-blocks Used Available Capacity Mounted on\n/dev/sdb 214643507200 1 1 1% /data"},
	}
	if got := inspectionStorageBytes(checks); got != 214643507200 {
		t.Fatalf("got %d, want one filesystem capacity", got)
	}
}

func TestInspectionFeishuPostContainsRealSummaryAndLink(t *testing.T) {
	run := InspectionRun{
		ID: 6, RunID: "20260630-163106", ClusterName: "simplehpc-dev", Status: "passed",
		PassedCount: 21, SkippedCount: 2, ProblemCount: 0, DurationMS: 290,
		Summary: map[string]any{"nodes": 1, "cpuCores": 20, "gpuCount": 0, "memoryGB": 29.296875, "storageBytes": int64(214643507200), "runningJobs": 0, "pendingJobs": 0, "users": 50},
		Checks:  []InspectionCheck{{Name: "GPU 设备", Category: "GPU", Status: "skipped", Message: "未配置 GPU"}},
	}
	payload := BuildInspectionFeishuPost(run, "http://10.10.38.152:8080/api/v1/inspection/runs/6/report")
	raw := fmt.Sprint(payload)
	for _, expected := range []string{"msg_type:post", "simplehpc-dev", "20", "未配置 GPU", "在线查看完整报告"} {
		if !strings.Contains(raw, expected) {
			t.Fatalf("payload missing %q: %s", expected, raw)
		}
	}
}

func TestInspectionSummaryPassesWhenEveryCheckIsOK(t *testing.T) {
	checks := []InspectionCheck{
		{Name: "PostgreSQL", Status: "ok"},
		{Name: "Slurm", Status: "ok"},
	}
	if got := summarizeInspection(checks); got != "passed" {
		t.Fatalf("got %q, want passed", got)
	}
}
