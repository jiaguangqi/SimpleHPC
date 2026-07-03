package slurm

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var qosNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)

type QOSAssignmentRequest struct {
	Account     string   `json:"account"`
	Users       []string `json:"users"`
	FallbackQOS string   `json:"fallbackQos"`
}

type QOSAssignmentResult struct {
	Account string   `json:"account"`
	QOS     string   `json:"qos"`
	Users   []string `json:"users"`
	Applied int      `json:"applied"`
}

func (c *Client) SaveQOS(ctx context.Context, original string, item QOS) error {
	if err := validateQOS(item); err != nil {
		return err
	}
	if original != "" && original != item.Name {
		return errors.New("QOS 名称创建后不可修改")
	}
	if original == "" {
		if _, err := c.run(ctx, "sacctmgr", "-i", "add", "qos", item.Name); err != nil && !strings.Contains(strings.ToLower(err.Error()), "nothing new added") {
			return err
		}
	}
	args := []string{"-i", "modify", "qos", "where", "name=" + item.Name, "set"}
	fields := []struct{ name, value string }{
		{"Description", item.Description}, {"MaxJobsPA", item.MaxJobsPA}, {"MaxJobsPU", item.MaxJobsPU},
		{"MaxSubmitJobsPA", item.MaxSubmitJobsPA}, {"MaxSubmitJobsPU", item.MaxSubmitJobsPU},
		{"MaxTRESPA", item.MaxTRESPA}, {"MaxTRESPU", item.MaxTRESPU}, {"MaxWall", item.MaxWall},
		{"MaxTRESPerNode", item.MaxTRESPerNode}, {"GrpJobs", item.GrpJobs}, {"GrpSubmit", item.GrpSubmit},
		{"GrpTRES", item.GrpTRES}, {"GrpWall", item.GrpWall}, {"Priority", item.Priority},
	}
	for _, field := range fields {
		if strings.TrimSpace(field.value) != "" {
			args = append(args, field.name+"="+strings.TrimSpace(field.value))
		}
	}
	if len(args) == 6 {
		return nil
	}
	_, err := c.run(ctx, "sacctmgr", args...)
	return err
}

func (c *Client) DeleteQOS(ctx context.Context, name string) error {
	if !qosNamePattern.MatchString(name) {
		return errors.New("QOS 名称格式不合法")
	}
	if strings.EqualFold(name, "normal") {
		return errors.New("不能删除 Slurm 默认 normal QOS")
	}
	_, err := c.run(ctx, "sacctmgr", "-i", "delete", "qos", "where", "name="+name)
	return err
}

func (c *Client) AssignQOS(ctx context.Context, qosName string, request QOSAssignmentRequest) (QOSAssignmentResult, error) {
	if !qosNamePattern.MatchString(qosName) {
		return QOSAssignmentResult{}, errors.New("QOS 名称格式不合法")
	}
	account := strings.TrimSpace(request.Account)
	if !qosNamePattern.MatchString(account) {
		return QOSAssignmentResult{}, errors.New("Slurm Account 格式不合法")
	}
	fallback := strings.TrimSpace(request.FallbackQOS)
	if fallback == "" {
		fallback = "normal"
	}
	if !qosNamePattern.MatchString(fallback) {
		return QOSAssignmentResult{}, errors.New("回退 QOS 格式不合法")
	}
	users := uniqueValidNames(request.Users)
	if len(users) != len(request.Users) {
		return QOSAssignmentResult{}, errors.New("用户账号包含不支持的字符或重复项")
	}

	if _, err := c.run(ctx, "sacctmgr", "-i", "add", "account", account); err != nil && !isNothingAdded(err) {
		return QOSAssignmentResult{}, fmt.Errorf("创建 Slurm Account 失败: %w", err)
	}
	if _, err := c.run(ctx, "sacctmgr", "-i", "modify", "account", "where", "name="+account, "set", "qos+="+qosName); err != nil {
		return QOSAssignmentResult{}, fmt.Errorf("下发 Account QOS 失败: %w", err)
	}
	for _, user := range users {
		if _, err := c.run(ctx, "sacctmgr", "-i", "add", "user", user, "account="+account); err != nil && !isNothingAdded(err) {
			return QOSAssignmentResult{}, fmt.Errorf("创建用户 %s 的 Slurm 关联失败: %w", user, err)
		}
		if _, err := c.run(ctx, "sacctmgr", "-i", "modify", "user", "where", "name="+user, "account="+account, "set", "qos+="+qosName, "defaultqos="+qosName); err != nil {
			return QOSAssignmentResult{}, fmt.Errorf("下发用户 %s 的 QOS 失败: %w", user, err)
		}
	}
	return QOSAssignmentResult{Account: account, QOS: qosName, Users: users, Applied: len(users)}, nil
}

func validateQOS(item QOS) error {
	if !qosNamePattern.MatchString(strings.TrimSpace(item.Name)) {
		return errors.New("QOS 名称仅允许字母、数字、点、下划线和连字符")
	}
	values := []string{item.MaxJobsPA, item.MaxJobsPU, item.MaxSubmitJobsPA, item.MaxSubmitJobsPU, item.Priority}
	for _, value := range values {
		if value != "" && !regexp.MustCompile(`^[0-9]+$`).MatchString(value) {
			return errors.New("作业数量和优先级必须为非负整数")
		}
	}
	for _, value := range []string{item.MaxTRESPA, item.MaxTRESPU, item.MaxTRESPerNode, item.GrpTRES, item.MaxWall, item.GrpWall} {
		if strings.ContainsAny(value, ";\n\r") {
			return errors.New("资源限制字段包含不支持的字符")
		}
	}
	return nil
}

func uniqueValidNames(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if !qosNamePattern.MatchString(value) || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func isNothingAdded(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "nothing new added")
}
