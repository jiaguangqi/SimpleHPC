package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSecureOpenWithinRootRejectsSymbolicLink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if file, err := secureOpenWithinRoot(root, filepath.Join(link, "secret.txt"), os.O_RDONLY, 0); err == nil {
		file.Close()
		t.Fatal("secure open followed a symbolic link outside the storage root")
	}
}

func TestSecureOpenWithinRootCreatesRegularFile(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "new.txt")
	file, err := secureOpenWithinRoot(root, target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("ok"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(target); err != nil || string(got) != "ok" {
		t.Fatalf("created content=%q err=%v", got, err)
	}
}
