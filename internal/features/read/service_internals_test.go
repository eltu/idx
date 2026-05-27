package read

import (
	"os"
	"path/filepath"
	"testing"

	sharedfs "idx/internal/shared/filesystem"
	sharedout "idx/internal/shared/output"
)

func evalSymlinks(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	return resolved
}

// --- noopReadLog ---

func TestNoopReadLogLoadAllReturnsNil(t *testing.T) {
	repo := noopReadLog{}
	entries, err := repo.LoadAll("/any/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entries != nil {
		t.Errorf("expected nil entries, got %v", entries)
	}
}

func TestNoopReadLogRecordReadReturnsNil(t *testing.T) {
	repo := noopReadLog{}
	if err := repo.RecordRead("/project", "file.go"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- findProjectRoot ---

func TestFindProjectRootLocatesGitDir(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "internal", "core")
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o750); err != nil {
		t.Fatalf("failed to create .git: %v", err)
	}
	if err := os.MkdirAll(nested, 0o750); err != nil {
		t.Fatalf("failed to create nested dir: %v", err)
	}

	projectTree := sharedfs.NewOSProjectTree()
	svc := NewReadCommandService(projectTree, NewOSFileStreamer(), sharedout.NewLineWriter(nil))

	// change to nested dir
	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(nested); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	gotRoot, err := svc.findProjectRoot()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// macOS resolves /var → /private/var via symlink; compare canonical paths.
	if evalSymlinks(t, gotRoot) != evalSymlinks(t, root) {
		t.Errorf("expected %q, got %q", root, gotRoot)
	}
}

// --- recordReadAccess (via RunWithOptions) ---

func TestRecordReadAccessSkipsGitSystemPaths(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	if err := os.MkdirAll(gitDir, 0o750); err != nil {
		t.Fatalf("create .git: %v", err)
	}
	gitFile := filepath.Join(gitDir, "HEAD")
	if err := os.WriteFile(gitFile, []byte("ref: refs/heads/main\n"), 0o600); err != nil {
		t.Fatalf("write HEAD: %v", err)
	}

	var recorded []string
	logRepo := &recordingReadLog{onRecord: func(p, r string) { recorded = append(recorded, r) }}

	projectTree := sharedfs.NewOSProjectTree()
	svc := NewReadCommandService(projectTree, NewOSFileStreamer(), sharedout.NewLineWriter(nil)).
		WithReadLog(logRepo)

	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	_ = os.Chdir(root)

	// Reading a .git file should not record the access
	_ = svc.RunWithOptions(gitFile, 0, 0)
	if len(recorded) > 0 {
		t.Errorf("expected no read recorded for .git path, got: %v", recorded)
	}
}

type recordingReadLog struct {
	onRecord func(projectRoot, rel string)
}

func (r *recordingReadLog) RecordRead(projectRoot, rel string) error {
	r.onRecord(projectRoot, rel)
	return nil
}

func (r *recordingReadLog) LoadAll(_ string) ([]LogEntry, error) {
	return nil, nil
}
