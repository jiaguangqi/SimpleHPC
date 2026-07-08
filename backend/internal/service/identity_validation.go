package service

import (
	"fmt"
	"net/mail"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	linuxAccountNamePattern = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)
	unitCodePattern         = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,31}$`)
)

var linuxReservedNames = map[string]struct{}{
	"adm": {}, "apache": {}, "backup": {}, "bin": {}, "daemon": {}, "dbus": {},
	"games": {}, "gnats": {}, "irc": {}, "list": {}, "lp": {}, "mail": {},
	"man": {}, "messagebus": {}, "mysql": {}, "news": {}, "nginx": {},
	"nobody": {}, "operator": {}, "postgres": {}, "proxy": {}, "redis": {},
	"root": {}, "slurm": {}, "sshd": {}, "sync": {}, "sys": {},
	"systemd-network": {}, "systemd-resolve": {}, "uucp": {}, "wheel": {},
	"www-data": {},
}

func normalizeCreateUserInput(input CreateUserInput) CreateUserInput {
	input.Username = strings.TrimSpace(input.Username)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Email = strings.TrimSpace(input.Email)
	input.Team = strings.TrimSpace(input.Team)
	input.Unit = strings.TrimSpace(input.Unit)
	input.HomeDirectory = strings.TrimSpace(input.HomeDirectory)
	return input
}

func normalizeUpdateUserInput(input UpdateUserInput) UpdateUserInput {
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Email = strings.TrimSpace(input.Email)
	return input
}

func normalizeCreateTeamInput(input CreateTeamInput) CreateTeamInput {
	input.Name = strings.TrimSpace(input.Name)
	input.GroupName = strings.TrimSpace(input.GroupName)
	input.Unit = strings.TrimSpace(input.Unit)
	input.LeaderUsername = strings.TrimSpace(input.LeaderUsername)
	input.ResourcePolicy = strings.TrimSpace(input.ResourcePolicy)
	return input
}

func normalizeUnitInput(input UnitInput) UnitInput {
	input.Name = strings.TrimSpace(input.Name)
	input.Code = strings.TrimSpace(input.Code)
	input.Admin = strings.TrimSpace(input.Admin)
	input.Status = strings.TrimSpace(input.Status)
	return input
}

func normalizeAdminCreateInput(input AdminCreate) AdminCreate {
	input.Username = strings.TrimSpace(input.Username)
	input.Email = strings.TrimSpace(input.Email)
	input.RoleName = strings.TrimSpace(input.RoleName)
	return input
}

func validateLinuxAccountName(value string, label string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("%s不能为空", label)
	}
	if trimmed != value {
		return fmt.Errorf("%s不能包含首尾空格", label)
	}
	if !linuxAccountNamePattern.MatchString(trimmed) {
		return fmt.Errorf("%s只能使用小写字母、数字、下划线和中划线，必须以小写字母或下划线开头，长度 1-32 位", label)
	}
	if _, ok := linuxReservedNames[trimmed]; ok {
		return fmt.Errorf("%s %s 为系统保留名称，不能用于创建", label, trimmed)
	}
	return nil
}

func validateUnitCode(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("单位编码不能为空")
	}
	if trimmed != value {
		return fmt.Errorf("单位编码不能包含首尾空格")
	}
	if !unitCodePattern.MatchString(trimmed) {
		return fmt.Errorf("单位编码只能使用小写字母、数字、下划线和中划线，必须以小写字母开头，长度 2-32 位")
	}
	if _, ok := linuxReservedNames[trimmed]; ok {
		return fmt.Errorf("单位编码 %s 为系统保留名称，不能用于创建", trimmed)
	}
	return nil
}

func validateEmailAddress(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("邮箱不能为空")
	}
	parsed, err := mail.ParseAddress(trimmed)
	if err != nil || parsed.Address != trimmed {
		return fmt.Errorf("邮箱格式不正确")
	}
	return nil
}

func validateHomeDirectoryForUser(home string, username string) error {
	trimmed := strings.TrimSpace(home)
	if trimmed == "" {
		return nil
	}
	if trimmed != home {
		return fmt.Errorf("主目录不能包含首尾空格")
	}
	if strings.ContainsRune(home, 0) {
		return fmt.Errorf("主目录包含非法字符")
	}
	if !filepath.IsAbs(home) {
		return fmt.Errorf("主目录必须是规范的绝对路径")
	}
	clean := filepath.Clean(home)
	if clean == "/" || clean != home {
		return fmt.Errorf("主目录必须是规范的绝对路径")
	}
	for _, part := range strings.Split(clean, string(filepath.Separator)) {
		if part == ".." {
			return fmt.Errorf("主目录不能包含目录穿越")
		}
	}
	return nil
}
