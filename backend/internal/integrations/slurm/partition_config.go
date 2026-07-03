package slurm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	partitionNamePattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)
	partitionValuePattern = regexp.MustCompile(`^[A-Za-z0-9_.,:@+\-\[\]*]+$`)
	partitionWriteMu      sync.Mutex
)

type PartitionConfig struct {
	Name           string `json:"name"`
	Nodes          string `json:"nodes"`
	MaxTime        string `json:"maxTime"`
	MaxCPUsPerNode string `json:"maxCPUsPerNode"`
	AllowGroups    string `json:"allowGroups"`
	AllowAccounts  string `json:"allowAccounts"`
	AllowQOS       string `json:"allowQos"`
	Default        string `json:"default"`
	State          string `json:"state"`
	QOS            string `json:"qos"`
	TotalCPUs      string `json:"totalCpus,omitempty"`
	TotalNodes     string `json:"totalNodes,omitempty"`
}

type PublishResult struct {
	BackupPath string   `json:"backupPath"`
	ConfigPath string   `json:"configPath"`
	Reloaded   bool     `json:"reloaded"`
	Nodes      []string `json:"nodes"`
}

func (c *Client) PartitionConfigs(ctx context.Context) ([]PartitionConfig, error) {
	out, err := c.run(ctx, "scontrol", "show", "partition", "-o")
	if err != nil {
		return nil, err
	}
	items := make([]PartitionConfig, 0)
	for _, line := range splitLines(out) {
		values := parseKeyValueLine(line)
		items = append(items, PartitionConfig{
			Name: values["PartitionName"], Nodes: values["Nodes"],
			MaxTime: values["MaxTime"], MaxCPUsPerNode: values["MaxCPUsPerNode"],
			AllowGroups: values["AllowGroups"], AllowAccounts: values["AllowAccounts"],
			AllowQOS: values["AllowQos"], Default: values["Default"],
			State: values["State"], QOS: normalizeNone(values["QoS"]),
			TotalCPUs: values["TotalCPUs"], TotalNodes: values["TotalNodes"],
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

func (c *Client) SavePartition(ctx context.Context, originalName string, item PartitionConfig) (PublishResult, error) {
	if err := validatePartitionConfig(item); err != nil {
		return PublishResult{}, err
	}
	if originalName != "" && !partitionNamePattern.MatchString(originalName) {
		return PublishResult{}, errors.New("原分区名称格式不合法")
	}
	partitionWriteMu.Lock()
	defer partitionWriteMu.Unlock()

	raw, info, err := readConfigFile(c.ConfigPath)
	if err != nil {
		return PublishResult{}, err
	}
	lines := strings.Split(string(raw), "\n")
	matchName := originalName
	if matchName == "" {
		matchName = item.Name
	}
	found := false
	for i, line := range lines {
		lineName := partitionNameFromLine(line)
		if item.Default == "YES" && lineName != "" && lineName != matchName {
			lines[i] = setConfigToken(line, "Default", "NO")
		}
		if lineName != matchName {
			continue
		}
		lines[i] = mergePartitionConfigLine(line, item)
		found = true
	}
	if originalName == "" && found {
		return PublishResult{}, fmt.Errorf("分区 %s 已存在", item.Name)
	}
	if originalName != "" && !found {
		return PublishResult{}, fmt.Errorf("分区 %s 不存在", originalName)
	}
	if !found {
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
			lines = append(lines, "")
		}
		lines = append(lines, renderPartitionConfig(item))
	}
	return c.publishConfig(ctx, raw, []byte(strings.Join(lines, "\n")), info.Mode())
}

func (c *Client) DeletePartition(ctx context.Context, name string) (PublishResult, error) {
	if !partitionNamePattern.MatchString(name) {
		return PublishResult{}, errors.New("分区名称格式不合法")
	}
	partitionWriteMu.Lock()
	defer partitionWriteMu.Unlock()

	raw, info, err := readConfigFile(c.ConfigPath)
	if err != nil {
		return PublishResult{}, err
	}
	lines := strings.Split(string(raw), "\n")
	output := make([]string, 0, len(lines))
	found := false
	for _, line := range lines {
		if partitionNameFromLine(line) == name {
			if strings.EqualFold(parseKeyValueLine(line)["Default"], "YES") {
				return PublishResult{}, errors.New("不能直接删除默认分区，请先将其他分区设为默认分区")
			}
			found = true
			continue
		}
		output = append(output, line)
	}
	if !found {
		return PublishResult{}, fmt.Errorf("分区 %s 不存在", name)
	}
	return c.publishConfig(ctx, raw, []byte(strings.Join(output, "\n")), info.Mode())
}

func (c *Client) publishConfig(ctx context.Context, previous, candidate []byte, mode os.FileMode) (PublishResult, error) {
	if strings.TrimSpace(c.ConfigPath) == "" {
		return PublishResult{}, errors.New("未配置 Slurm 配置文件路径")
	}
	backup := c.ConfigPath + ".simplehpc-" + time.Now().Format("20060102-150405") + ".bak"
	if err := os.WriteFile(backup, previous, mode); err != nil {
		return PublishResult{}, fmt.Errorf("创建配置备份失败: %w", err)
	}
	if err := atomicWrite(c.ConfigPath, candidate, mode); err != nil {
		return PublishResult{}, fmt.Errorf("写入候选配置失败: %w", err)
	}
	if _, err := c.run(ctx, "scontrol", "reconfigure"); err != nil {
		_ = atomicWrite(c.ConfigPath, previous, mode)
		_, _ = c.run(context.Background(), "scontrol", "reconfigure")
		return PublishResult{}, fmt.Errorf("Slurm 拒绝新配置，已恢复原配置: %w", err)
	}
	nodes, _ := c.Nodes(ctx)
	nodeNames := make([]string, 0, len(nodes))
	seen := map[string]bool{}
	for _, node := range nodes {
		if node.Name != "" && !seen[node.Name] {
			seen[node.Name] = true
			nodeNames = append(nodeNames, node.Name)
		}
	}
	sort.Strings(nodeNames)
	return PublishResult{BackupPath: backup, ConfigPath: c.ConfigPath, Reloaded: true, Nodes: nodeNames}, nil
}

func validatePartitionConfig(item PartitionConfig) error {
	if !partitionNamePattern.MatchString(strings.TrimSpace(item.Name)) {
		return errors.New("PartitionName 仅允许字母、数字、下划线、点和连字符")
	}
	fields := map[string]string{
		"Nodes": item.Nodes, "MaxTime": item.MaxTime, "MaxCPUsPerNode": item.MaxCPUsPerNode,
		"AllowGroups": item.AllowGroups, "AllowAccounts": item.AllowAccounts,
		"AllowQos": item.AllowQOS, "QOS": item.QOS,
	}
	for name, value := range fields {
		value = strings.TrimSpace(value)
		if value != "" && !partitionValuePattern.MatchString(value) {
			return fmt.Errorf("%s 包含不支持的字符", name)
		}
	}
	if item.Nodes == "" {
		return errors.New("Nodes 不能为空")
	}
	if item.Default != "YES" && item.Default != "NO" {
		return errors.New("Default 必须为 YES 或 NO")
	}
	if item.State != "UP" && item.State != "DOWN" && item.State != "INACTIVE" && item.State != "DRAIN" {
		return errors.New("State 取值不合法")
	}
	return nil
}

func renderPartitionConfig(item PartitionConfig) string {
	fields := []string{"PartitionName=" + item.Name, "Nodes=" + item.Nodes}
	appendValue := func(name, value string) {
		if value = strings.TrimSpace(value); value != "" && value != "N/A" && value != "NONE" {
			fields = append(fields, name+"="+value)
		}
	}
	appendValue("MaxTime", item.MaxTime)
	appendValue("MaxCPUsPerNode", item.MaxCPUsPerNode)
	appendValue("AllowGroups", item.AllowGroups)
	appendValue("AllowAccounts", item.AllowAccounts)
	appendValue("AllowQos", item.AllowQOS)
	fields = append(fields, "Default="+item.Default, "State="+item.State)
	appendValue("QOS", item.QOS)
	return strings.Join(fields, " ")
}

func mergePartitionConfigLine(line string, item PartitionConfig) string {
	updates := []struct {
		key   string
		value string
	}{
		{"PartitionName", item.Name}, {"Nodes", item.Nodes}, {"MaxTime", item.MaxTime},
		{"MaxCPUsPerNode", item.MaxCPUsPerNode}, {"AllowGroups", item.AllowGroups},
		{"AllowAccounts", item.AllowAccounts}, {"AllowQos", item.AllowQOS},
		{"Default", item.Default}, {"State", item.State}, {"QOS", item.QOS},
	}
	result := line
	for _, update := range updates {
		result = setConfigToken(result, update.key, update.value)
	}
	return strings.TrimSpace(result)
}

func setConfigToken(line, key, value string) string {
	tokens := strings.Fields(strings.TrimSpace(line))
	match := key + "="
	found := false
	output := make([]string, 0, len(tokens)+1)
	for _, token := range tokens {
		if !strings.HasPrefix(token, match) {
			output = append(output, token)
			continue
		}
		found = true
		if value != "" && value != "N/A" && value != "NONE" {
			output = append(output, match+value)
		}
	}
	if !found && value != "" && value != "N/A" && value != "NONE" {
		output = append(output, match+value)
	}
	return strings.Join(output, " ")
}

func parseKeyValueLine(line string) map[string]string {
	values := map[string]string{}
	for _, token := range strings.Fields(line) {
		if parts := strings.SplitN(token, "=", 2); len(parts) == 2 {
			values[parts[0]] = parts[1]
		}
	}
	return values
}

func normalizeNone(value string) string {
	if value == "N/A" || value == "(null)" {
		return ""
	}
	return value
}

func partitionNameFromLine(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return ""
	}
	for _, token := range strings.Fields(trimmed) {
		if strings.HasPrefix(token, "PartitionName=") {
			return strings.TrimPrefix(token, "PartitionName=")
		}
	}
	return ""
}

func readConfigFile(path string) ([]byte, os.FileInfo, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("读取 Slurm 配置失败: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, nil, err
	}
	return raw, info, nil
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".simplehpc-slurm-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
