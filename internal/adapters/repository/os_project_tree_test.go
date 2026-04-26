package repository

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOSProjectTreeCurrentDirAndFindGitRoot(t *testing.T) {
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("expected cwd read to succeed, got %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalDir) })

	root := t.TempDir()
	gitPath := filepath.Join(root, ".git")
	if err := os.WriteFile(gitPath, []byte(""), 0600); err != nil {
		t.Fatalf("expected git marker creation to succeed, got %v", err)
	}

	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(nested, 0750); err != nil {
		t.Fatalf("expected nested directory creation to succeed, got %v", err)
	}

	if err := os.Chdir(nested); err != nil {
		t.Fatalf("expected chdir to succeed, got %v", err)
	}

	tree := NewOSProjectTree()
	cwd, err := tree.CurrentDir()
	if err != nil {
		t.Fatalf("expected CurrentDir to succeed, got %v", err)
	}
	resolvedExpected, err := filepath.EvalSymlinks(nested)
	if err != nil {
		t.Fatalf("expected expected-path symlink resolution to succeed, got %v", err)
	}
	resolvedCurrent, err := filepath.EvalSymlinks(cwd)
	if err != nil {
		t.Fatalf("expected current-path symlink resolution to succeed, got %v", err)
	}
	if resolvedCurrent != resolvedExpected {
		t.Fatalf("expected cwd %q, got %q", resolvedExpected, resolvedCurrent)
	}

	gitRoot, err := tree.FindGitRoot(nested)
	if err != nil {
		t.Fatalf("expected FindGitRoot to succeed, got %v", err)
	}
	if gitRoot != root {
		t.Fatalf("expected root %q, got %q", root, gitRoot)
	}
}

func TestOSProjectTreeFindGitRootReturnsErrorOutsideGitProject(t *testing.T) {
	tree := NewOSProjectTree()
	_, err := tree.FindGitRoot(t.TempDir())
	if err == nil {
		t.Fatal("expected error when directory is outside git project, got nil")
	}
}

func TestOSProjectTreeReadDirExistsRemoveAllAndWriteFile(t *testing.T) {
	tree := NewOSProjectTree()
	root := t.TempDir()
	filePath := filepath.Join(root, "deep", "file.txt")

	if err := tree.WriteFile(filePath, []byte("content")); err != nil {
		t.Fatalf("expected write to succeed, got %v", err)
	}

	exists, err := tree.Exists(filePath)
	if err != nil {
		t.Fatalf("expected exists to succeed, got %v", err)
	}
	if !exists {
		t.Fatal("expected file to exist")
	}

	entries, err := tree.ReadDir(filepath.Join(root, "deep"))
	if err != nil {
		t.Fatalf("expected readdir to succeed, got %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "file.txt" || entries[0].IsDir {
		t.Fatalf("unexpected read dir entries: %#v", entries)
	}

	if entries[0].Size == 0 {
		t.Fatal("expected file size metadata to be populated")
	}
	if entries[0].ModTimeUnixNano == 0 {
		t.Fatal("expected file modtime metadata to be populated")
	}

	if err := tree.RemoveAll(filepath.Join(root, "deep")); err != nil {
		t.Fatalf("expected remove all to succeed, got %v", err)
	}

	exists, err = tree.Exists(filePath)
	if err != nil {
		t.Fatalf("expected exists after remove to succeed, got %v", err)
	}
	if exists {
		t.Fatal("expected file to be removed")
	}
}

func TestOSProjectTreeErrorBranches(t *testing.T) {
	tree := NewOSProjectTree()

	if _, err := tree.ReadDir(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("expected ReadDir error for missing path")
	}

	if _, err := tree.Exists("\x00invalid"); err == nil {
		t.Fatal("expected Exists error for invalid path")
	}

	if err := tree.RemoveAll("\x00invalid"); err == nil {
		t.Fatal("expected RemoveAll error for invalid path")
	}

	if err := tree.WriteFile("\x00invalid", []byte("x")); err == nil {
		t.Fatal("expected WriteFile error for invalid path")
	}
}
