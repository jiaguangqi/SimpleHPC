package slurm

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGPUCountFromGRES(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "empty", value: "", want: "0"},
		{name: "null", value: "(null)", want: "0"},
		{name: "generic gpu", value: "gpu:4", want: "4"},
		{name: "typed gpu", value: "gpu:a100:8", want: "8"},
		{name: "allocated index suffix", value: "gpu:h100:2(IDX:0-1)", want: "2"},
		{name: "multiple gpu resources", value: "gpu:a100:2,gpu:v100:4", want: "6"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := gpuCountFromGRES(tt.value); got != tt.want {
				t.Fatalf("gpuCountFromGRES(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestValidateJobID(t *testing.T) {
	valid := []string{"928", "1284593_4", "1284593_4.batch", "1284593+1"}
	for _, value := range valid {
		if err := validateJobID(value); err != nil {
			t.Fatalf("validateJobID(%q) returned %v", value, err)
		}
	}
	invalid := []string{"", "928; shutdown", "../928", "928 929"}
	for _, value := range invalid {
		if err := validateJobID(value); err == nil {
			t.Fatalf("validateJobID(%q) accepted unsafe value", value)
		}
	}
}

func TestJobActionsInvokeSlurmCommands(t *testing.T) {
	binDir := t.TempDir()
	logPath := filepath.Join(binDir, "commands.log")
	script := "#!/bin/sh\nprintf '%s %s\\n' \"$(basename \"$0\")\" \"$*\" >> \"" + logPath + "\"\n"
	for _, command := range []string{"scancel", "scontrol"} {
		if err := os.WriteFile(filepath.Join(binDir, command), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	client := New(binDir, "", "", "")
	ctx := context.Background()
	if err := client.CancelJob(ctx, "928"); err != nil {
		t.Fatal(err)
	}
	if err := client.SuspendJob(ctx, "929"); err != nil {
		t.Fatal(err)
	}
	if err := client.ResumeJob(ctx, "930"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimSpace(string(data)), "\n")
	want := []string{"scancel 928", "scontrol suspend 929", "scontrol resume 930"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("commands = %#v, want %#v", got, want)
	}
}

func TestHistoryIncludesNodeList(t *testing.T) {
	binDir := t.TempDir()
	sacctPath := filepath.Join(binDir, "sacct")
	script := "#!/bin/sh\nprintf '%s\\n' '931|wrap|root|debug|root|1|1|COMPLETED|00:10:00|2026-06-28T00:00:00|2026-06-28T00:00:01|2026-06-28T00:10:01|cae'\n"
	if err := os.WriteFile(sacctPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	jobs, err := New(binDir, "", "", "").History(context.Background(), "2026-06-28")
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 {
		t.Fatalf("History() returned %d jobs, want 1", len(jobs))
	}
	if jobs[0].NodeList != "cae" {
		t.Fatalf("History()[0].NodeList = %q, want %q", jobs[0].NodeList, "cae")
	}
}

func TestJobDetailReturnsAccountingFields(t *testing.T) {
	binDir := t.TempDir()
	sacctPath := filepath.Join(binDir, "sacct")
	script := "#!/bin/sh\nprintf '%s\\n' '935|job.sh|root|simplehpc|debug|normal|COMPLETED|1|1|1|30000M|billing=1,cpu=1,mem=30000M,node=1|2026-06-29T16:10:20|2026-06-29T16:10:20|2026-06-29T16:27:01|00:16:41|cae|/data/simpleHPC|slurm-%j.out|slurm-%j.err'\n"
	if err := os.WriteFile(sacctPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	detail, err := New(binDir, "", "", "").JobDetail(context.Background(), "935")
	if err != nil {
		t.Fatal(err)
	}
	if detail.CPUs != "1" || detail.Requested != "billing=1,cpu=1,mem=30000M,node=1" {
		t.Fatalf("JobDetail() resources = CPUs %q, requested %q", detail.CPUs, detail.Requested)
	}
	if detail.Account != "simplehpc" {
		t.Fatalf("JobDetail().Account = %q, want simplehpc", detail.Account)
	}
	if detail.Start != "2026-06-29T16:10:20" || detail.End != "2026-06-29T16:27:01" {
		t.Fatalf("JobDetail() times = start %q, end %q", detail.Start, detail.End)
	}
	if detail.WorkDir != "/data/simpleHPC" {
		t.Fatalf("JobDetail().WorkDir = %q", detail.WorkDir)
	}
	if detail.StdOut != "slurm-%j.out" || detail.StdErr != "slurm-%j.err" {
		t.Fatalf("JobDetail() output paths = %q, %q", detail.StdOut, detail.StdErr)
	}
}

func TestJobOutputReadsExpandedPathInsideWorkDir(t *testing.T) {
	binDir := t.TempDir()
	workDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workDir, "slurm-938.out"), []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sacctPath := filepath.Join(binDir, "sacct")
	row := "938|Shell|root|simplehpc|debug|normal|RUNNING|1|1|1|30000M|billing=1,cpu=1,mem=30000M,node=1|2026-06-29T19:54:51|2026-06-29T22:33:11|Unknown|00:37:40|cae|" + workDir + "|slurm-%j.out|slurm-%j.err"
	script := "#!/bin/sh\nprintf '%s\\n' '" + row + "'\n"
	if err := os.WriteFile(sacctPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	output, err := New(binDir, "", "", "").JobOutput(context.Background(), "938", "stdout")
	if err != nil {
		t.Fatal(err)
	}
	if output.Content != "hello world\n" || output.Path != filepath.Join(workDir, "slurm-938.out") {
		t.Fatalf("JobOutput() = %#v", output)
	}
}

func TestParseSubmittedJobID(t *testing.T) {
	for input, want := range map[string]string{"932\n": "932", "933;simplehpc\n": "933"} {
		got, err := parseSubmittedJobID(input)
		if err != nil || got != want {
			t.Fatalf("parseSubmittedJobID(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
	if _, err := parseSubmittedJobID("not-a-job"); err == nil {
		t.Fatal("parseSubmittedJobID accepted invalid output")
	}
}
