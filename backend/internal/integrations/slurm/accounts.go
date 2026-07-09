package slurm

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var regexpLinuxUser = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)

type ProjectAccountRequest struct {
	Name         string
	Parent       string
	Description  string
	Organization string
	QOS          string
}

func validateSlurmAccountName(name string, label string) error {
	if !qosNamePattern.MatchString(strings.TrimSpace(name)) {
		return fmt.Errorf("%s 仅允许字母、数字、点、下划线和连字符，且不能超过 64 位", label)
	}
	return nil
}

func (c *Client) EnsureProjectAccount(ctx context.Context, request ProjectAccountRequest) error {
	account := strings.TrimSpace(request.Name)
	if err := validateSlurmAccountName(account, "Slurm Account"); err != nil {
		return err
	}
	parent := strings.TrimSpace(request.Parent)
	if parent != "" {
		if err := validateSlurmAccountName(parent, "父级 Slurm Account"); err != nil {
			return err
		}
	}
	qos := strings.TrimSpace(request.QOS)
	if qos != "" {
		if err := validateSlurmAccountName(qos, "Slurm QOS"); err != nil {
			return err
		}
	}
	args := []string{"-i", "add", "account", account}
	if parent != "" {
		args = append(args, "parent="+parent)
	}
	if description := cleanSlurmTextArg(request.Description); description != "" {
		args = append(args, "Description="+description)
	}
	if org := cleanSlurmTextArg(request.Organization); org != "" {
		args = append(args, "Organization="+org)
	}
	if _, err := c.run(ctx, "sacctmgr", args...); err != nil && !isNothingAdded(err) {
		return fmt.Errorf("创建 Slurm Account 失败: %w", err)
	}
	if qos != "" {
		if _, err := c.run(ctx, "sacctmgr", "-i", "modify", "account", "where", "name="+account, "set", "qos+="+qos); err != nil {
			return fmt.Errorf("绑定 Slurm Account QOS 失败: %w", err)
		}
	}
	return nil
}

func (c *Client) EnsureUserAccountAssociation(ctx context.Context, account string, username string, qos string, defaultAccount bool) error {
	account = strings.TrimSpace(account)
	username = strings.TrimSpace(username)
	qos = strings.TrimSpace(qos)
	if err := validateSlurmAccountName(account, "Slurm Account"); err != nil {
		return err
	}
	if !regexpLinuxUser.MatchString(username) {
		return errors.New("用户账号格式不合法，无法创建 Slurm Account 关联")
	}
	if qos != "" {
		if err := validateSlurmAccountName(qos, "Slurm QOS"); err != nil {
			return err
		}
	}
	if _, err := c.run(ctx, "sacctmgr", "-i", "add", "user", username, "account="+account); err != nil && !isNothingAdded(err) {
		return fmt.Errorf("创建用户 %s 的 Slurm Account 关联失败: %w", username, err)
	}
	setArgs := []string{"-i", "modify", "user", "where", "name=" + username, "account=" + account, "set"}
	if qos != "" {
		setArgs = append(setArgs, "qos+="+qos, "defaultqos="+qos)
	}
	if defaultAccount {
		setArgs = append(setArgs, "defaultaccount="+account)
	}
	if len(setArgs) > 7 {
		if _, err := c.run(ctx, "sacctmgr", setArgs...); err != nil {
			return fmt.Errorf("更新用户 %s 的 Slurm Account 关联失败: %w", username, err)
		}
	}
	return nil
}

func cleanSlurmTextArg(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	if len([]rune(value)) > 120 {
		value = string([]rune(value)[:120])
	}
	return value
}
