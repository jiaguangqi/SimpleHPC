package slurm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var slurmJobIDPattern = regexp.MustCompile(`^[0-9]+(?:_[0-9]+)?(?:\.[A-Za-z0-9_-]+)?(?:\+[0-9]+)?$`)

type Client struct {
	BinDir           string
	ConfigPath       string
	DefaultAccount   string
	DefaultPartition string
}

type Node struct {
	Name         string `json:"name"`
	Partition    string `json:"partition"`
	State        string `json:"state"`
	CPUs         string `json:"cpus"`
	CPUAllocated string `json:"cpuAllocated,omitempty"`
	CPUIdle      string `json:"cpuIdle,omitempty"`
	CPUOther     string `json:"cpuOther,omitempty"`
	CPUTotal     string `json:"cpuTotal,omitempty"`
	MemoryMB     string `json:"memoryMB"`
	GRES         string `json:"gres,omitempty"`
	Reason       string `json:"reason,omitempty"`
}

type Job struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	User      string `json:"user"`
	Partition string `json:"partition"`
	State     string `json:"state"`
	Nodes     string `json:"nodes"`
	CPUs      string `json:"cpus"`
	GPUs      string `json:"gpus"`
	Time      string `json:"time"`
	Submit    string `json:"submit,omitempty"`
	NodeList  string `json:"nodeList,omitempty"`
}

type HistoricalJob struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	User      string `json:"user"`
	Partition string `json:"partition"`
	Account   string `json:"account"`
	Nodes     string `json:"nodes"`
	CPUs      string `json:"cpus"`
	State     string `json:"state"`
	Elapsed   string `json:"elapsed"`
	Submit    string `json:"submit"`
	Start     string `json:"start"`
	End       string `json:"end"`
	NodeList  string `json:"nodeList,omitempty"`
}

type JobDetail struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	User      string `json:"user"`
	Partition string `json:"partition"`
	QOS       string `json:"qos"`
	State     string `json:"state"`
	Nodes     string `json:"nodes"`
	CPUs      string `json:"cpus"`
	ReqCPUs   string `json:"reqCpus"`
	ReqMem    string `json:"reqMem"`
	Requested string `json:"requested"`
	Submit    string `json:"submit"`
	Start     string `json:"start"`
	End       string `json:"end"`
	Elapsed   string `json:"elapsed"`
	NodeList  string `json:"nodeList"`
	WorkDir   string `json:"workdir"`
	StdOut    string `json:"stdout"`
	StdErr    string `json:"stderr"`
}

type JobOutput struct {
	Stream    string `json:"stream"`
	Path      string `json:"path"`
	Content   string `json:"content"`
	Size      int64  `json:"size"`
	Truncated bool   `json:"truncated"`
	Exists    bool   `json:"exists"`
}

type Partition struct {
	Name         string `json:"name"`
	Availability string `json:"availability"`
	NodeList     string `json:"nodeList"`
	CPUsPerNode  string `json:"cpusPerNode"`
	MemoryMB     string `json:"memoryMB"`
	GRES         string `json:"gres"`
	MaxTime      string `json:"maxTime"`
}

type QOS struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	MaxJobsPA       string `json:"maxJobsPA"`
	MaxJobsPU       string `json:"maxJobsPU"`
	MaxSubmitJobsPA string `json:"maxSubmitJobsPA"`
	MaxSubmitJobsPU string `json:"maxSubmitJobsPU"`
	MaxTRESPA       string `json:"maxTRESPA"`
	MaxTRESPU       string `json:"maxTRESPU"`
	MaxWall         string `json:"maxWall"`
	MaxTRESPerNode  string `json:"maxTRESPerNode"`
	GrpJobs         string `json:"grpJobs"`
	GrpSubmit       string `json:"grpSubmit"`
	GrpTRES         string `json:"grpTRES"`
	GrpWall         string `json:"grpWall"`
	Priority        string `json:"priority"`
}

type Association struct {
	User       string `json:"user"`
	Account    string `json:"account"`
	Partition  string `json:"partition"`
	QOS        string `json:"qos"`
	DefaultQOS string `json:"defaultQos"`
}

type Account struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Org         string `json:"org"`
	QOS         string `json:"qos"`
}

func New(binDir, configPath, defaultAccount, defaultPartition string) *Client {
	return &Client{BinDir: binDir, ConfigPath: configPath, DefaultAccount: defaultAccount, DefaultPartition: defaultPartition}
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.run(ctx, "sinfo", "--version")
	return err
}

func (c *Client) Nodes(ctx context.Context) ([]Node, error) {
	out, err := c.run(ctx, "sinfo", "-N", "-h", "-o", "%N|%P|%T|%C|%m|%G|%E")
	if err != nil {
		return nil, err
	}
	rows := splitLines(out)
	nodes := make([]Node, 0, len(rows))
	for _, row := range rows {
		cols := splitRow(row, 7)
		cpuParts := splitRow(strings.ReplaceAll(cols[3], "/", "|"), 4)
		nodes = append(nodes, Node{
			Name:         cols[0],
			Partition:    strings.TrimSuffix(cols[1], "*"),
			State:        cols[2],
			CPUs:         cols[3],
			CPUAllocated: cpuParts[0],
			CPUIdle:      cpuParts[1],
			CPUOther:     cpuParts[2],
			CPUTotal:     cpuParts[3],
			MemoryMB:     cols[4],
			GRES:         cols[5],
			Reason:       cols[6],
		})
	}
	return nodes, nil
}

func (c *Client) Jobs(ctx context.Context) ([]Job, error) {
	out, err := c.run(ctx, "squeue", "-h", "-o", "%i|%j|%u|%P|%T|%D|%C|%M|%V|%R|%b")
	if err != nil {
		return nil, err
	}
	rows := splitLines(out)
	jobs := make([]Job, 0, len(rows))
	for _, row := range rows {
		cols := splitRow(row, 11)
		jobs = append(jobs, Job{
			ID: cols[0], Name: cols[1], User: cols[2], Partition: cols[3],
			State: cols[4], Nodes: cols[5], CPUs: cols[6], Time: cols[7], Submit: cols[8],
			GPUs: gpuCountFromGRES(cols[10]), NodeList: cols[9],
		})
	}
	return jobs, nil
}

func validateJobID(jobID string) error {
	if !slurmJobIDPattern.MatchString(strings.TrimSpace(jobID)) {
		return errors.New("无效的 Slurm 作业号")
	}
	return nil
}

func (c *Client) CancelJob(ctx context.Context, jobID string) error {
	if err := validateJobID(jobID); err != nil {
		return err
	}
	_, err := c.run(ctx, "scancel", strings.TrimSpace(jobID))
	return err
}

func (c *Client) SuspendJob(ctx context.Context, jobID string) error {
	if err := validateJobID(jobID); err != nil {
		return err
	}
	_, err := c.run(ctx, "scontrol", "suspend", strings.TrimSpace(jobID))
	return err
}

func (c *Client) ResumeJob(ctx context.Context, jobID string) error {
	if err := validateJobID(jobID); err != nil {
		return err
	}
	_, err := c.run(ctx, "scontrol", "resume", strings.TrimSpace(jobID))
	return err
}

func parseSubmittedJobID(output string) (string, error) {
	value := strings.TrimSpace(strings.Split(strings.TrimSpace(output), ";")[0])
	if err := validateJobID(value); err != nil {
		return "", fmt.Errorf("无法解析 sbatch 返回的作业号: %q", strings.TrimSpace(output))
	}
	return value, nil
}

func (c *Client) SubmitScript(ctx context.Context, script, username string) (string, error) {
	username = strings.TrimSpace(username)
	if !regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`).MatchString(username) {
		return "", errors.New("无效的 Linux 用户名")
	}
	tempDir, err := os.MkdirTemp("", "simplehpc-job-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempDir)
	path := filepath.Join(tempDir, "job.sh")
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		return "", err
	}
	_ = os.Chmod(tempDir, 0o755)
	_ = os.Chmod(path, 0o755)
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	sbatch := filepath.Join(c.BinDir, "sbatch")
	var cmd *exec.Cmd
	if username == "root" {
		cmd = exec.CommandContext(ctx, sbatch, "--parsable", path)
	} else {
		cmd = exec.CommandContext(ctx, "runuser", "-u", username, "--", sbatch, "--parsable", path)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return "", errors.New(message)
	}
	return parseSubmittedJobID(string(output))
}

func gpuCountFromGRES(value string) string {
	total := 0
	for _, token := range strings.Split(value, ",") {
		lower := strings.ToLower(strings.TrimSpace(token))
		if !strings.Contains(lower, "gpu") {
			continue
		}
		parts := strings.Split(lower, ":")
		for i := len(parts) - 1; i >= 0; i-- {
			part := strings.TrimSpace(strings.TrimSuffix(parts[i], ")"))
			if n := leadingInteger(part); n > 0 {
				total += n
				break
			}
		}
	}
	return strconv.Itoa(total)
}

func leadingInteger(value string) int {
	end := 0
	for end < len(value) && value[end] >= '0' && value[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0
	}
	n, _ := strconv.Atoi(value[:end])
	return n
}

func (c *Client) History(ctx context.Context, since string) ([]HistoricalJob, error) {
	if since == "" {
		since = "today"
	}
	out, err := c.run(ctx, "sacct", "-S", since, "--parsable2", "--noheader", "-X", "--format=JobID,JobName,User,Partition,Account,NNodes,AllocCPUS,State,Elapsed,Submit,Start,End,NodeList")
	if err != nil {
		return nil, err
	}
	rows := splitLines(out)
	jobs := make([]HistoricalJob, 0, len(rows))
	for _, row := range rows {
		cols := splitRow(row, 13)
		jobs = append(jobs, HistoricalJob{
			ID: cols[0], Name: cols[1], User: cols[2], Partition: cols[3], Account: cols[4],
			Nodes: cols[5], CPUs: cols[6], State: cols[7], Elapsed: cols[8],
			Submit: cols[9], Start: cols[10], End: cols[11], NodeList: cols[12],
		})
	}
	return jobs, nil
}

func (c *Client) JobDetail(ctx context.Context, jobID string) (JobDetail, error) {
	if err := validateJobID(jobID); err != nil {
		return JobDetail{}, err
	}
	out, err := c.run(ctx, "sacct", "-j", strings.TrimSpace(jobID), "-X", "--parsable2", "--noheader",
		"--format=JobID,JobName,User,Partition,QOS,State,NNodes,AllocCPUS,ReqCPUS,ReqMem,ReqTRES,Submit,Start,End,Elapsed,NodeList,WorkDir,StdOut,StdErr")
	if err != nil {
		return JobDetail{}, err
	}
	rows := splitLines(out)
	if len(rows) == 0 {
		return JobDetail{}, errors.New("Slurm 中未找到该作业")
	}
	cols := splitRow(rows[0], 19)
	return JobDetail{
		ID: cols[0], Name: cols[1], User: cols[2], Partition: cols[3], QOS: cols[4],
		State: cols[5], Nodes: cols[6], CPUs: cols[7], ReqCPUs: cols[8], ReqMem: cols[9],
		Requested: cols[10], Submit: cols[11], Start: cols[12], End: cols[13],
		Elapsed: cols[14], NodeList: cols[15], WorkDir: cols[16], StdOut: cols[17], StdErr: cols[18],
	}, nil
}

func (c *Client) JobOutput(ctx context.Context, jobID, stream string) (JobOutput, error) {
	stream = strings.ToLower(strings.TrimSpace(stream))
	if stream != "stdout" && stream != "stderr" {
		return JobOutput{}, errors.New("输出类型必须是 stdout 或 stderr")
	}
	detail, err := c.JobDetail(ctx, jobID)
	if err != nil {
		return JobOutput{}, err
	}
	outputPath := detail.StdOut
	if stream == "stderr" {
		outputPath = detail.StdErr
	}
	result := JobOutput{Stream: stream}
	if strings.TrimSpace(outputPath) == "" {
		return result, nil
	}
	outputPath = strings.ReplaceAll(outputPath, "%j", strings.TrimSpace(jobID))
	if !filepath.IsAbs(outputPath) {
		outputPath = filepath.Join(detail.WorkDir, outputPath)
	}
	outputPath = filepath.Clean(outputPath)
	workDir := filepath.Clean(detail.WorkDir)
	relative, err := filepath.Rel(workDir, outputPath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return JobOutput{}, errors.New("输出文件不在作业工作目录内")
	}
	result.Path = outputPath
	info, err := os.Stat(outputPath)
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return JobOutput{}, err
	}
	const maxOutputBytes int64 = 2 << 20
	start := int64(0)
	if info.Size() > maxOutputBytes {
		start = info.Size() - maxOutputBytes
		result.Truncated = true
	}
	file, err := os.Open(outputPath)
	if err != nil {
		return JobOutput{}, err
	}
	defer file.Close()
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return JobOutput{}, err
	}
	content, err := io.ReadAll(io.LimitReader(file, maxOutputBytes))
	if err != nil {
		return JobOutput{}, err
	}
	result.Content, result.Size, result.Exists = string(content), info.Size(), true
	return result, nil
}

func (c *Client) Partitions(ctx context.Context) ([]Partition, error) {
	out, err := c.run(ctx, "sinfo", "-h", "-o", "%P|%a|%N|%c|%m|%G|%l")
	if err != nil {
		return nil, err
	}
	rows := splitLines(out)
	partitions := make([]Partition, 0, len(rows))
	for _, row := range rows {
		cols := splitRow(row, 7)
		partitions = append(partitions, Partition{
			Name:         strings.TrimSuffix(cols[0], "*"),
			Availability: cols[1],
			NodeList:     cols[2],
			CPUsPerNode:  cols[3],
			MemoryMB:     cols[4],
			GRES:         cols[5],
			MaxTime:      cols[6],
		})
	}
	return partitions, nil
}

func (c *Client) QOS(ctx context.Context) ([]QOS, error) {
	out, err := c.run(ctx, "sacctmgr", "-n", "-P", "show", "qos", "format=Name,Description,MaxJobsPA,MaxJobsPU,MaxSubmitJobsPA,MaxSubmitJobsPU,MaxTRESPA,MaxTRESPU,MaxWall,MaxTRESPerNode,GrpJobs,GrpSubmit,GrpTRES,GrpWall,Priority")
	if err != nil {
		return nil, err
	}
	rows := splitLines(out)
	qosItems := make([]QOS, 0, len(rows))
	for _, row := range rows {
		cols := splitRow(row, 15)
		qosItems = append(qosItems, QOS{
			Name: cols[0], Description: cols[1], MaxJobsPA: cols[2], MaxJobsPU: cols[3],
			MaxSubmitJobsPA: cols[4], MaxSubmitJobsPU: cols[5], MaxTRESPA: cols[6],
			MaxTRESPU: cols[7], MaxWall: cols[8], MaxTRESPerNode: cols[9],
			GrpJobs: cols[10], GrpSubmit: cols[11], GrpTRES: cols[12],
			GrpWall: cols[13], Priority: cols[14],
		})
	}
	return qosItems, nil
}

func (c *Client) Associations(ctx context.Context) ([]Association, error) {
	out, err := c.run(ctx, "sacctmgr", "-n", "-P", "show", "user", "withassoc", "format=User,Account,Partition,QOS,DefaultQOS")
	if err != nil {
		return nil, err
	}
	items := make([]Association, 0)
	for _, row := range splitLines(out) {
		cols := splitRow(row, 5)
		items = append(items, Association{User: cols[0], Account: cols[1], Partition: cols[2], QOS: cols[3], DefaultQOS: cols[4]})
	}
	return items, nil
}

func (c *Client) Accounts(ctx context.Context) ([]Account, error) {
	out, err := c.run(ctx, "sacctmgr", "-n", "-P", "show", "account", "format=Account,Descr,Org,QOS")
	if err != nil {
		return nil, err
	}
	rows := splitLines(out)
	accounts := make([]Account, 0, len(rows))
	for _, row := range rows {
		cols := splitRow(row, 4)
		accounts = append(accounts, Account{Name: cols[0], Description: cols[1], Org: cols[2], QOS: cols[3]})
	}
	return accounts, nil
}

func (c *Client) run(ctx context.Context, name string, args ...string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	bin := filepath.Join(c.BinDir, name)
	cmd := exec.CommandContext(ctx, bin, args...)
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", err
		}
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return "", errors.New(message)
	}
	return string(output), nil
}

func splitLines(text string) []string {
	raw := strings.Split(strings.TrimSpace(text), "\n")
	rows := make([]string, 0, len(raw))
	for _, row := range raw {
		if trimmed := strings.TrimSpace(row); trimmed != "" {
			rows = append(rows, trimmed)
		}
	}
	return rows
}

func splitRow(row string, width int) []string {
	cols := strings.Split(row, "|")
	out := make([]string, width)
	for i := 0; i < width; i++ {
		if i < len(cols) {
			out[i] = strings.TrimSpace(cols[i])
		}
	}
	return out
}
