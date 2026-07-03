package service

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var linuxUserHomeSSHKeygen = "ssh-keygen"

type linuxUserHomeInitResult struct {
	HomeCreated bool
}

func ensureLinuxUserHome(ctx context.Context, username, home string, uid, gid int) (linuxUserHomeInitResult, error) {
	result := linuxUserHomeInitResult{}
	username = strings.TrimSpace(username)
	home = strings.TrimSpace(home)
	if username == "" {
		return result, fmt.Errorf("账号不能为空")
	}
	if strings.ContainsAny(username, "/\x00") || strings.Contains(username, "..") {
		return result, fmt.Errorf("账号包含非法字符")
	}
	if home == "" || !filepath.IsAbs(home) {
		return result, fmt.Errorf("用户主目录必须是绝对路径")
	}
	if uid <= 0 || gid <= 0 {
		return result, fmt.Errorf("用户 UID/GID 无效")
	}
	if _, err := os.Stat(home); os.IsNotExist(err) {
		result.HomeCreated = true
	} else if err != nil {
		return result, err
	}
	if err := os.MkdirAll(home, 0700); err != nil {
		return result, err
	}
	if err := os.Chown(home, uid, gid); err != nil {
		return result, err
	}
	if err := os.Chmod(home, 0700); err != nil {
		return result, err
	}
	if err := ensureShellStartupFiles(home, uid, gid); err != nil {
		return result, err
	}
	if err := ensureSSHTrust(ctx, home, uid, gid); err != nil {
		return result, err
	}
	return result, nil
}

func cleanupCreatedHome(result linuxUserHomeInitResult, home string) {
	if result.HomeCreated {
		_ = os.RemoveAll(home)
	}
}

func ensureShellStartupFiles(home string, uid, gid int) error {
	files := map[string]string{
		".bash_profile": "# .bash_profile\n\nif [ -f ~/.bashrc ]; then\n    . ~/.bashrc\nfi\n",
		".bashrc":       "# .bashrc\n\nif [ -f /etc/bashrc ]; then\n    . /etc/bashrc\nfi\n",
		".bash_logout":  "# .bash_logout\n",
	}
	for name, fallback := range files {
		path := filepath.Join(home, name)
		if _, err := os.Stat(path); err == nil {
			if err := os.Chown(path, uid, gid); err != nil {
				return err
			}
			continue
		} else if !os.IsNotExist(err) {
			return err
		}
		data := []byte(fallback)
		if skelData, err := os.ReadFile(filepath.Join("/etc/skel", name)); err == nil && len(bytes.TrimSpace(skelData)) > 0 {
			data = skelData
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			return err
		}
		if err := os.Chown(path, uid, gid); err != nil {
			return err
		}
	}
	return nil
}

func ensureSSHTrust(ctx context.Context, home string, uid, gid int) error {
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return err
	}
	if err := os.Chown(sshDir, uid, gid); err != nil {
		return err
	}
	if err := os.Chmod(sshDir, 0700); err != nil {
		return err
	}

	privateKey := filepath.Join(sshDir, "id_rsa")
	publicKey := privateKey + ".pub"
	if _, err := os.Stat(privateKey); os.IsNotExist(err) {
		cmd := exec.CommandContext(ctx, linuxUserHomeSSHKeygen, "-t", "rsa", "-b", "3072", "-N", "", "-f", privateKey)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("ssh-keygen: %s", strings.TrimSpace(string(output)))
		}
	} else if err != nil {
		return err
	}
	if _, err := os.Stat(publicKey); os.IsNotExist(err) {
		cmd := exec.CommandContext(ctx, linuxUserHomeSSHKeygen, "-y", "-f", privateKey)
		output, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("ssh-keygen -y: %w", err)
		}
		if err := os.WriteFile(publicKey, append(bytes.TrimSpace(output), '\n'), 0644); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	pub, err := os.ReadFile(publicKey)
	if err != nil {
		return err
	}
	if err := ensureAuthorizedKey(filepath.Join(sshDir, "authorized_keys"), string(bytes.TrimSpace(pub))); err != nil {
		return err
	}
	sshConfig := "Host *\n    StrictHostKeyChecking no\n    UserKnownHostsFile /dev/null\n    GlobalKnownHostsFile /dev/null\n    LogLevel ERROR\n"
	if err := os.WriteFile(filepath.Join(sshDir, "config"), []byte(sshConfig), 0600); err != nil {
		return err
	}
	for _, item := range []struct {
		path string
		mode os.FileMode
	}{
		{privateKey, 0600},
		{publicKey, 0644},
		{filepath.Join(sshDir, "authorized_keys"), 0600},
		{filepath.Join(sshDir, "config"), 0600},
	} {
		if err := os.Chmod(item.path, item.mode); err != nil {
			return err
		}
		if err := os.Chown(item.path, uid, gid); err != nil {
			return err
		}
	}
	return os.Chown(sshDir, uid, gid)
}

func ensureAuthorizedKey(path, key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("SSH 公钥为空")
	}
	existing, err := os.ReadFile(path)
	if err == nil {
		for _, line := range strings.Split(string(existing), "\n") {
			if strings.TrimSpace(line) == key {
				return nil
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	if len(existing) > 0 && !bytes.HasSuffix(existing, []byte("\n")) {
		if _, err := file.WriteString("\n"); err != nil {
			return err
		}
	}
	_, err = file.WriteString(key + "\n")
	return err
}
