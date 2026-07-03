package service

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestEnsureLinuxUserHomeCreatesShellFilesAndSSHTrust(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix file modes required")
	}
	tmp := t.TempDir()
	fakeKeygen := filepath.Join(tmp, "ssh-keygen")
	script := `#!/bin/sh
out=""
extract=0
while [ "$#" -gt 0 ]; do
  if [ "$1" = "-y" ]; then extract=1; fi
  if [ "$1" = "-f" ]; then shift; out="$1"; fi
  shift
done
if [ "$extract" = "1" ]; then
  echo "ssh-rsa TESTKEY"
  exit 0
fi
printf "PRIVATE\n" > "$out"
printf "ssh-rsa TESTKEY\n" > "$out.pub"
`
	if err := os.WriteFile(fakeKeygen, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	original := linuxUserHomeSSHKeygen
	linuxUserHomeSSHKeygen = fakeKeygen
	t.Cleanup(func() { linuxUserHomeSSHKeygen = original })

	home := filepath.Join(tmp, "home", "youyou")
	result, err := ensureLinuxUserHome(context.Background(), "youyou", home, os.Getuid(), os.Getgid())
	if err != nil {
		t.Fatal(err)
	}
	if !result.HomeCreated {
		t.Fatal("expected new home to be marked as created")
	}
	for _, name := range []string{".bash_profile", ".bashrc", ".bash_logout"} {
		if _, err := os.Stat(filepath.Join(home, name)); err != nil {
			t.Fatalf("%s missing: %v", name, err)
		}
	}
	auth, err := os.ReadFile(filepath.Join(home, ".ssh", "authorized_keys"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(auth), "ssh-rsa TESTKEY") {
		t.Fatalf("authorized_keys missing generated public key: %q", string(auth))
	}
	config, err := os.ReadFile(filepath.Join(home, ".ssh", "config"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(config), "StrictHostKeyChecking no") ||
		!strings.Contains(string(config), "UserKnownHostsFile /dev/null") ||
		!strings.Contains(string(config), "GlobalKnownHostsFile /dev/null") ||
		!strings.Contains(string(config), "LogLevel ERROR") {
		t.Fatalf("ssh config does not disable host key prompts: %q", string(config))
	}
	assertMode(t, filepath.Join(home, ".ssh"), 0700)
	assertMode(t, filepath.Join(home, ".ssh", "id_rsa"), 0600)
	assertMode(t, filepath.Join(home, ".ssh", "authorized_keys"), 0600)
	assertMode(t, filepath.Join(home, ".ssh", "config"), 0600)
}

func TestEnsureLinuxUserHomeRejectsUnsafeInput(t *testing.T) {
	if _, err := ensureLinuxUserHome(context.Background(), "../bad", "/tmp/bad", 1000, 1000); err == nil {
		t.Fatal("expected unsafe username to be rejected")
	}
	if _, err := ensureLinuxUserHome(context.Background(), "user001", "relative/home", 1000, 1000); err == nil {
		t.Fatal("expected relative home to be rejected")
	}
	if _, err := ensureLinuxUserHome(context.Background(), "user001", "/tmp/user001", 0, 1000); err == nil {
		t.Fatal("expected invalid uid to be rejected")
	}
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("%s mode = %o, want %o", path, got, want)
	}
}
