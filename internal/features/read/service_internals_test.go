package read

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func TestNoopReadLog_LoadAll_ReturnsNil(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := noopReadLog{}

	// Act
	entries, err := repo.LoadAll("/any/path")

	// Assert
	require.NoError(t, err)
	assert.Nil(t, entries)
}

func TestNoopReadLog_RecordRead_ReturnsNil(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := noopReadLog{}

	// Act & Assert
	assert.NoError(t, repo.RecordRead("/project", "file.go"))
}

// --- findProjectRoot ---

// TestFindProjectRoot_LocatesGitDir uses os.Chdir — not parallel-safe.
func TestFindProjectRoot_LocatesGitDir(t *testing.T) {
	// Arrange
	root := t.TempDir()
	nested := filepath.Join(root, "internal", "core")
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0o750))
	require.NoError(t, os.MkdirAll(nested, 0o750))

	projectTree := sharedfs.NewOSProjectTree()
	svc := NewReadCommandService(projectTree, NewOSFileStreamer(), sharedout.NewLineWriter(nil))

	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	require.NoError(t, os.Chdir(nested))

	// Act
	gotRoot, err := svc.findProjectRoot()

	// Assert
	require.NoError(t, err)
	// macOS resolves /var → /private/var via symlink; compare canonical paths.
	assert.Equal(t, evalSymlinks(t, root), evalSymlinks(t, gotRoot))
}

// --- recordReadAccess (via RunWithOptions) ---

// TestRecordReadAccess_SkipsGitSystemPaths uses os.Chdir — not parallel-safe.
func TestRecordReadAccess_SkipsGitSystemPaths(t *testing.T) {
	// Arrange
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0o750))
	gitFile := filepath.Join(gitDir, "HEAD")
	require.NoError(t, os.WriteFile(gitFile, []byte("ref: refs/heads/main\n"), 0o600))

	var recorded []string
	logRepo := &recordingReadLog{onRecord: func(p, r string) { recorded = append(recorded, r) }}

	projectTree := sharedfs.NewOSProjectTree()
	svc := NewReadCommandService(projectTree, NewOSFileStreamer(), sharedout.NewLineWriter(nil)).
		WithReadLog(logRepo)

	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	_ = os.Chdir(root)

	// Act — reading a .git file should not record the access
	_ = svc.RunWithOptions(gitFile, 0, 0)

	// Assert
	assert.Empty(t, recorded)
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
