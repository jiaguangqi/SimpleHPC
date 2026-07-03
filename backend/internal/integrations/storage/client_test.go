package storage

import (
	"archive/zip"
	"bytes"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestListHidesDotFilesByDefault(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "visible.txt"), "visible")
	mustWriteFile(t, filepath.Join(root, ".secret"), "secret")
	client := New([]string{root})

	entries, err := client.List(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name != "visible.txt" {
		t.Fatalf("expected only visible.txt, got %#v", entries)
	}

	entries, err = client.List(root, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected hidden file when requested, got %#v", entries)
	}
}

func TestPathNormalizationRejectsTraversalVariants(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	if err := os.Mkdir(child, 0o700); err != nil {
		t.Fatal(err)
	}
	client := New([]string{root})
	for _, allowed := range []string{
		root + string(os.PathSeparator) + string(os.PathSeparator) + "child",
		filepath.Join(root, ".", "child"),
	} {
		if _, err := client.List(allowed, false); err != nil {
			t.Fatalf("normalized safe path %q rejected: %v", allowed, err)
		}
	}
	encoded, err := url.QueryUnescape(filepath.ToSlash(filepath.Join(root, "%2e%2e", filepath.Base(t.TempDir()))))
	if err != nil {
		t.Fatal(err)
	}
	for _, denied := range []string{
		filepath.Join(root, ".."),
		filepath.Join(root, "..", "other"),
		encoded,
		"/etc",
	} {
		if _, err := client.List(denied, false); err == nil {
			t.Fatalf("unsafe path %q accepted", denied)
		}
	}
}

func TestArchiveRejectsNestedSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	dir := filepath.Join(root, "selected")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(outside, "secret.txt"), "secret")
	if err := os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(dir, "escape.txt")); err != nil {
		t.Fatal(err)
	}
	client := New([]string{root})
	var output bytes.Buffer
	if err := client.Archive([]string{dir}, &output); err == nil {
		t.Fatal("archive followed a nested symlink")
	}
}

func TestDeleteRejectsSymlinkInsideTree(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	dir := filepath.Join(root, "selected")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(outside, "secret.txt"), "secret")
	if err := os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(dir, "escape.txt")); err != nil {
		t.Fatal(err)
	}
	client := New([]string{root})
	if err := client.Delete([]string{dir}); err == nil {
		t.Fatal("delete accepted a tree containing a symlink")
	}
	if _, err := os.Stat(filepath.Join(outside, "secret.txt")); err != nil {
		t.Fatalf("outside file changed: %v", err)
	}
}

func TestCopyAndMoveRejectSymlinkInsideTree(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	copySource := filepath.Join(root, "copy-source")
	moveSource := filepath.Join(root, "move-source")
	dest := filepath.Join(root, "dest")
	if err := os.Mkdir(dest, 0o700); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(outside, "secret.txt"), "secret")
	for _, source := range []string{copySource, moveSource} {
		if err := os.MkdirAll(source, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(source, "escape.txt")); err != nil {
			t.Fatal(err)
		}
	}
	client := New([]string{root})
	if err := client.Copy([]string{copySource}, dest); err == nil {
		t.Fatal("copy accepted a tree containing a symlink")
	}
	if err := client.Move([]string{moveSource}, dest); err == nil {
		t.Fatal("move accepted a tree containing a symlink")
	}
	if _, err := os.Stat(filepath.Join(outside, "secret.txt")); err != nil {
		t.Fatalf("outside file changed: %v", err)
	}
}

func TestCopyAndMoveRejectSymlinkDestinationEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "copy.txt"), "copy")
	mustWriteFile(t, filepath.Join(root, "move.txt"), "move")
	linkDest := filepath.Join(root, "outside-link")
	if err := os.Symlink(outside, linkDest); err != nil {
		t.Fatal(err)
	}
	client := New([]string{root})
	if err := client.Copy([]string{filepath.Join(root, "copy.txt")}, linkDest); err == nil {
		t.Fatal("copy accepted a symlink destination outside the root")
	}
	if err := client.Move([]string{filepath.Join(root, "move.txt")}, linkDest); err == nil {
		t.Fatal("move accepted a symlink destination outside the root")
	}
	if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
		t.Fatalf("outside destination changed, entries=%v err=%v", entries, err)
	}
}

func TestArchiveNamesNeverContainTraversal(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "safe.txt"), "safe")
	client := New([]string{root})
	var output bytes.Buffer
	if err := client.Archive([]string{filepath.Join(root, "safe.txt")}, &output); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(output.Bytes()), int64(output.Len()))
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range reader.File {
		if strings.Contains(item.Name, "..") || strings.HasPrefix(item.Name, "/") {
			t.Fatalf("unsafe archive member %q", item.Name)
		}
	}
}

func TestCopyAndMoveRejectDestinationInsideSource(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	child := filepath.Join(source, "child")
	if err := os.MkdirAll(child, 0o700); err != nil {
		t.Fatal(err)
	}
	client := New([]string{root})
	if err := client.Copy([]string{source}, child); err == nil {
		t.Fatal("copy into source subtree was accepted")
	}
	if err := client.Move([]string{source}, child); err == nil {
		t.Fatal("move into source subtree was accepted")
	}
}

func TestRenameStaysInsideConfiguredRoot(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "old.txt"), "data")
	client := New([]string{root})
	if err := client.Rename(filepath.Join(root, "old.txt"), "new.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "new.txt")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../escape", "sub/escape", "/tmp/escape"} {
		if err := client.Rename(filepath.Join(root, "new.txt"), name); err == nil {
			t.Fatalf("unsafe rename %q was accepted", name)
		}
	}
}

func TestPathMetadataStopsAtInitialRoot(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	if err := os.Mkdir(child, 0o700); err != nil {
		t.Fatal(err)
	}
	client := New([]string{root})
	meta, err := client.PathMetadata(root)
	if err != nil {
		t.Fatal(err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if meta.CanGoParent || meta.ParentPath != "" || meta.InitialPath != resolvedRoot || meta.EffectivePath != resolvedRoot {
		t.Fatalf("root metadata = %#v", meta)
	}
	meta, err = client.PathMetadata(child)
	if err != nil {
		t.Fatal(err)
	}
	if !meta.CanGoParent || meta.InitialPath != resolvedRoot || meta.ParentPath != resolvedRoot {
		t.Fatalf("child metadata = %#v", meta)
	}
}

func TestClientRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	mustWriteFile(t, filepath.Join(outside, "secret.txt"), "secret")
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	client := New([]string{root})

	if _, err := client.List(filepath.Join(root, "escape"), false); err == nil {
		t.Fatal("expected symlink escape to be rejected")
	}
}

func TestFileOperationsStayInsideConfiguredRoot(t *testing.T) {
	root := t.TempDir()
	client := New([]string{root})

	if err := client.CreateDirectory(root, "new"); err != nil {
		t.Fatal(err)
	}
	if err := client.WriteFile(filepath.Join(root, "new"), "hello.txt", strings.NewReader("hello")); err != nil {
		t.Fatal(err)
	}
	if err := client.Copy([]string{filepath.Join(root, "new", "hello.txt")}, root); err != nil {
		t.Fatal(err)
	}
	if err := client.Move([]string{filepath.Join(root, "hello.txt")}, filepath.Join(root, "new")); err == nil {
		t.Fatal("expected move onto existing destination to fail")
	}
	if err := client.Delete([]string{filepath.Join(root, "new", "hello.txt")}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "new", "hello.txt")); !os.IsNotExist(err) {
		t.Fatalf("expected file to be deleted, got %v", err)
	}
}

func TestArchiveProducesZipWithSelectedFiles(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "a.txt"), "a")
	mustWriteFile(t, filepath.Join(root, "b.txt"), "b")
	client := New([]string{root})

	var output strings.Builder
	if err := client.Archive([]string{filepath.Join(root, "a.txt"), filepath.Join(root, "b.txt")}, &output); err != nil {
		t.Fatal(err)
	}
	if output.Len() == 0 {
		t.Fatal("expected non-empty zip output")
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o640); err != nil {
		t.Fatal(err)
	}
}
