package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const inspectionOutputLimit = 256 * 1024

type InspectionCommand struct {
	Name             string
	Category         string
	Command          string
	Args             []string
	SkipWhenMissing  bool
	UnavailableLabel string
}

type InspectionCheck struct {
	Name       string `json:"name"`
	Category   string `json:"category,omitempty"`
	Status     string `json:"status"`
	Message    string `json:"message,omitempty"`
	Command    string `json:"command,omitempty"`
	Host       string `json:"host,omitempty"`
	StartedAt  string `json:"startedAt,omitempty"`
	FinishedAt string `json:"finishedAt,omitempty"`
	DurationMS int64  `json:"durationMs"`
	ExitCode   int    `json:"exitCode"`
	Stdout     string `json:"stdout,omitempty"`
	Stderr     string `json:"stderr,omitempty"`
}

type InspectionRun struct {
	ID           int64             `json:"id"`
	RunID        string            `json:"runId"`
	Status       string            `json:"status"`
	ClusterName  string            `json:"clusterName"`
	Checks       []InspectionCheck `json:"checks,omitempty"`
	ProblemCount int               `json:"problemCount"`
	PassedCount  int               `json:"passedCount"`
	SkippedCount int               `json:"skippedCount"`
	CreatedBy    string            `json:"createdBy"`
	StartedAt    string            `json:"startedAt,omitempty"`
	FinishedAt   string            `json:"finishedAt,omitempty"`
	DurationMS   int64             `json:"durationMs"`
	CreatedAt    string            `json:"createdAt"`
	Summary      map[string]any    `json:"summary,omitempty"`
	ReportHTML   string            `json:"-"`
	DetailLog    string            `json:"-"`
}

func summarizeInspection(checks []InspectionCheck) string {
	for _, check := range checks {
		if check.Status == "error" {
			return "warning"
		}
	}
	return "passed"
}

func limitedText(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= inspectionOutputLimit {
		return value
	}
	return value[:inspectionOutputLimit] + "\n...[输出已截断]"
}

func runInspectionCommand(ctx context.Context, spec InspectionCommand) InspectionCheck {
	start := time.Now()
	result := InspectionCheck{
		Name: spec.Name, Category: spec.Category, Command: strings.TrimSpace(strings.Join(append([]string{spec.Command}, spec.Args...), " ")),
		StartedAt: start.Format(time.RFC3339Nano), ExitCode: -1,
	}
	path, err := exec.LookPath(spec.Command)
	if err != nil && filepath.IsAbs(spec.Command) {
		path = spec.Command
	}
	if err != nil {
		if spec.SkipWhenMissing {
			result.Status, result.Message = "skipped", spec.UnavailableLabel
		} else {
			result.Status, result.Message, result.Stderr = "error", err.Error(), err.Error()
		}
		result.FinishedAt, result.DurationMS = time.Now().Format(time.RFC3339Nano), time.Since(start).Milliseconds()
		return result
	}
	var stdout, stderr bytes.Buffer
	command := exec.CommandContext(ctx, path, spec.Args...)
	command.Stdout, command.Stderr = &stdout, &stderr
	err = command.Run()
	result.Stdout, result.Stderr = limitedText(stdout.String()), limitedText(stderr.String())
	result.ExitCode = 0
	result.Status, result.Message = "ok", "检查通过"
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
		}
		result.Status, result.Message = "error", err.Error()
	}
	result.FinishedAt, result.DurationMS = time.Now().Format(time.RFC3339Nano), time.Since(start).Milliseconds()
	return result
}

func logicalInspectionCheck(name, category, command string, start time.Time, err error) InspectionCheck {
	item := InspectionCheck{Name: name, Category: category, Command: command, StartedAt: start.Format(time.RFC3339Nano), FinishedAt: time.Now().Format(time.RFC3339Nano), ExitCode: 0, Status: "ok", Message: "检查通过", DurationMS: time.Since(start).Milliseconds()}
	if err != nil {
		item.Status, item.Message, item.Stderr, item.ExitCode = "error", err.Error(), err.Error(), 1
	}
	return item
}

func (s *Services) ExecuteInspection(ctx context.Context, username string) (InspectionRun, error) {
	start := time.Now()
	host, _ := runOutput(ctx, "/bin/hostname")
	slurm := func(name string, args ...string) InspectionCommand {
		return InspectionCommand{Name: name, Category: "Slurm 调度", Command: filepath.Join(s.Config.SlurmBinDir, args[0]), Args: args[1:]}
	}
	specs := []InspectionCommand{
		{Name: "主机运行时间与负载", Category: "操作系统", Command: "/usr/bin/uptime"},
		{Name: "系统内存", Category: "操作系统", Command: "/usr/bin/free", Args: []string{"-m"}},
		{Name: "关键服务状态", Category: "平台服务", Command: "/usr/bin/systemctl", Args: []string{"is-active", "simplehpc-backend", "slurmctld", "slurmd"}},
		slurm("Slurm 控制器", "scontrol", "ping"),
		slurm("Slurm 配置", "scontrol", "show", "config"),
		slurm("计算节点", "sinfo", "-N", "-h", "-o", "%N|%P|%T|%C|%m|%G|%E"),
		slurm("分区状态", "sinfo", "-h", "-o", "%P|%a|%l|%D|%C|%G"),
		slurm("当前作业队列", "squeue", "-h", "-o", "%i|%u|%T|%P|%C|%m|%M|%R"),
		slurm("调度诊断", "sdiag"),
		slurm("近七天作业", "sacct", "-X", "-S", "now-7days", "-n", "-P", "-o", "JobIDRaw,User,State,ElapsedRaw,AllocCPUS,ReqMem"),
		{Name: "GPU 设备", Category: "GPU", Command: "nvidia-smi", Args: []string{"--query-gpu=index,name,temperature.gpu,utilization.gpu,memory.total,memory.used,ecc.errors.uncorrected.volatile", "--format=csv,noheader,nounits"}, SkipWhenMissing: true, UnavailableLabel: "当前集群未安装 NVIDIA 工具或未配置 GPU"},
		{Name: "InfiniBand 端口", Category: "高速网络", Command: "ibstat", SkipWhenMissing: true, UnavailableLabel: "当前集群未安装 InfiniBand 工具或未配置 IB 网络"},
	}
	for _, root := range s.Config.StorageRoots {
		specs = append(specs,
			InspectionCommand{Name: "存储容量 " + root, Category: "存储系统", Command: "/usr/bin/df", Args: []string{"-P", "-B1", root}},
			InspectionCommand{Name: "inode 使用率 " + root, Category: "存储系统", Command: "/usr/bin/df", Args: []string{"-P", "-i", root}},
		)
	}
	checks := make([]InspectionCheck, 0, len(specs)+3)
	for _, spec := range specs {
		item := runInspectionCommand(ctx, spec)
		item.Host = strings.TrimSpace(host)
		checks = append(checks, item)
	}
	dbStart := time.Now()
	checks = append(checks, logicalInspectionCheck("PostgreSQL", "平台服务", "SELECT 1", dbStart, s.CheckPostgres(ctx)))
	redisStart := time.Now()
	checks = append(checks, logicalInspectionCheck("Redis", "平台服务", "PING", redisStart, s.CheckRedis(ctx)))
	ldapStart := time.Now()
	checks = append(checks, logicalInspectionCheck("OpenLDAP", "用户与权限", "LDAP bind", ldapStart, s.LDAP.Ping()))

	run := InspectionRun{RunID: start.Format("20060102-150405.000000"), Checks: checks, CreatedBy: username, StartedAt: start.Format(time.RFC3339)}
	run.Status = summarizeInspection(checks)
	for _, check := range checks {
		switch check.Status {
		case "ok":
			run.PassedCount++
		case "skipped":
			run.SkippedCount++
		default:
			run.ProblemCount++
		}
	}
	run.Summary, run.ClusterName = s.inspectionSummary(ctx, checks)
	finished := time.Now()
	run.FinishedAt, run.DurationMS = finished.Format(time.RFC3339), finished.Sub(start).Milliseconds()
	run.DetailLog = RenderInspectionLog(run)
	run.ReportHTML = RenderInspectionHTML(run)
	rawChecks, _ := json.Marshal(run.Checks)
	rawSummary, _ := json.Marshal(run.Summary)
	var created time.Time
	err := s.DB.QueryRowContext(ctx, `
INSERT INTO inspection_runs(run_id,status,checks,problem_count,created_by,started_at,finished_at,duration_ms,cluster_name,summary,report_html,detail_log)
VALUES($1,$2,$3::jsonb,$4,$5,$6,$7,$8,$9,$10::jsonb,$11,$12)
RETURNING id,created_at`, run.RunID, run.Status, string(rawChecks), run.ProblemCount, run.CreatedBy, start, finished, run.DurationMS, run.ClusterName, string(rawSummary), run.ReportHTML, run.DetailLog).Scan(&run.ID, &created)
	if err != nil {
		return run, err
	}
	run.CreatedAt = created.Format(time.RFC3339)
	return run, nil
}

func runOutput(ctx context.Context, name string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	return string(out), err
}

func (s *Services) inspectionSummary(ctx context.Context, checks []InspectionCheck) (map[string]any, string) {
	summary := map[string]any{"nodes": 0, "cpuCores": 0, "gpuCount": 0, "memoryGB": float64(0), "storageBytes": int64(0), "runningJobs": 0, "pendingJobs": 0, "users": 0}
	cluster := "数据未获取"
	for _, check := range checks {
		if check.Name == "Slurm 配置" {
			for _, line := range strings.Split(check.Stdout, "\n") {
				if strings.HasPrefix(strings.TrimSpace(line), "ClusterName") {
					parts := strings.SplitN(line, "=", 2)
					if len(parts) == 2 {
						cluster = strings.TrimSpace(parts[1])
					}
				}
			}
		}
		if check.Name == "计算节点" {
			nodes, cpus, memory, gpus := 0, 0, 0, 0
			for _, line := range strings.Split(strings.TrimSpace(check.Stdout), "\n") {
				cols := strings.Split(line, "|")
				if len(cols) < 6 {
					continue
				}
				nodes++
				cpu := strings.Split(cols[3], "/")
				if len(cpu) == 4 {
					value, _ := strconv.Atoi(cpu[3])
					cpus += value
				}
				value, _ := strconv.Atoi(cols[4])
				memory += value
				if strings.Contains(cols[5], "gpu:") {
					parts := strings.Split(cols[5], ":")
					value, _ := strconv.Atoi(parts[len(parts)-1])
					gpus += value
				}
			}
			summary["nodes"], summary["cpuCores"], summary["memoryGB"], summary["gpuCount"] = nodes, cpus, float64(memory)/1024, gpus
		}
		if check.Name == "当前作业队列" {
			running, pending := 0, 0
			for _, line := range strings.Split(strings.TrimSpace(check.Stdout), "\n") {
				cols := strings.Split(line, "|")
				if len(cols) < 3 {
					continue
				}
				if cols[2] == "RUNNING" {
					running++
				}
				if cols[2] == "PENDING" {
					pending++
				}
			}
			summary["runningJobs"], summary["pendingJobs"] = running, pending
		}
	}
	summary["storageBytes"] = inspectionStorageBytes(checks)
	var users int
	if s.DB.QueryRowContext(ctx, `SELECT count(*) FROM platform_users WHERE status<>'deleted'`).Scan(&users) == nil {
		summary["users"] = users
	}
	return summary, cluster
}

func inspectionStorageBytes(checks []InspectionCheck) int64 {
	seen := map[string]bool{}
	var total int64
	for _, check := range checks {
		if !strings.HasPrefix(check.Name, "存储容量 ") {
			continue
		}
		lines := strings.Split(strings.TrimSpace(check.Stdout), "\n")
		if len(lines) < 2 {
			continue
		}
		fields := strings.Fields(lines[len(lines)-1])
		if len(fields) < 2 || seen[fields[0]] {
			continue
		}
		value, err := strconv.ParseInt(fields[1], 10, 64)
		if err == nil {
			seen[fields[0]] = true
			total += value
		}
	}
	return total
}

func (s *Services) InspectionRuns(ctx context.Context, limit int) ([]InspectionRun, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT id,run_id,status,checks,problem_count,created_by,created_at,COALESCE(started_at,created_at),COALESCE(finished_at,created_at),duration_ms,cluster_name,summary FROM inspection_runs ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []InspectionRun{}
	for rows.Next() {
		var item InspectionRun
		var rawChecks, rawSummary []byte
		var created, started, finished time.Time
		if err := rows.Scan(&item.ID, &item.RunID, &item.Status, &rawChecks, &item.ProblemCount, &item.CreatedBy, &created, &started, &finished, &item.DurationMS, &item.ClusterName, &rawSummary); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(rawChecks, &item.Checks)
		_ = json.Unmarshal(rawSummary, &item.Summary)
		for _, check := range item.Checks {
			if check.Status == "ok" {
				item.PassedCount++
			}
			if check.Status == "skipped" {
				item.SkippedCount++
			}
		}
		item.CreatedAt, item.StartedAt, item.FinishedAt = created.Format(time.RFC3339), started.Format(time.RFC3339), finished.Format(time.RFC3339)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Services) InspectionRunByID(ctx context.Context, id int64) (InspectionRun, error) {
	var item InspectionRun
	var rawChecks, rawSummary []byte
	var created, started, finished time.Time
	err := s.DB.QueryRowContext(ctx, `SELECT id,run_id,status,checks,problem_count,created_by,created_at,COALESCE(started_at,created_at),COALESCE(finished_at,created_at),duration_ms,cluster_name,summary,report_html,detail_log FROM inspection_runs WHERE id=$1`, id).
		Scan(&item.ID, &item.RunID, &item.Status, &rawChecks, &item.ProblemCount, &item.CreatedBy, &created, &started, &finished, &item.DurationMS, &item.ClusterName, &rawSummary, &item.ReportHTML, &item.DetailLog)
	if err != nil {
		return item, err
	}
	_ = json.Unmarshal(rawChecks, &item.Checks)
	_ = json.Unmarshal(rawSummary, &item.Summary)
	for _, check := range item.Checks {
		if check.Status == "ok" {
			item.PassedCount++
		}
		if check.Status == "skipped" {
			item.SkippedCount++
		}
	}
	item.CreatedAt, item.StartedAt, item.FinishedAt = created.Format(time.RFC3339), started.Format(time.RFC3339), finished.Format(time.RFC3339)
	return item, nil
}

func formatBytes(value int64) string {
	const unit = int64(1024)
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}
	div, exp := unit, 0
	for n := value / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(value)/float64(div), "KMGTPE"[exp])
}
