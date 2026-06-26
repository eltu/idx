package search

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitChangedFiles_ValidRef_ReturnsChangedFiles(t *testing.T) {
	t.Parallel()

	// Arrange
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)
	writeGitFile(t, tmpDir, "a.go", "package a")
	runGit(t, tmpDir, "add", "a.go")
	runGit(t, tmpDir, "commit", "-m", "initial")
	writeGitFile(t, tmpDir, "b.go", "package b")
	runGit(t, tmpDir, "add", "b.go")
	runGit(t, tmpDir, "commit", "-m", "add b")

	// Act
	files, err := gitChangedFiles(tmpDir, "HEAD~1")

	// Assert
	require.NoError(t, err)
	assert.True(t, files["b.go"], "b.go should be in changed files")
	assert.False(t, files["a.go"], "a.go should not be in changed files")
}

func TestGitChangedFiles_InvalidRef_ReturnsErrorWithRef(t *testing.T) {
	t.Parallel()

	// Arrange
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)
	writeGitFile(t, tmpDir, "a.go", "package a")
	runGit(t, tmpDir, "add", "a.go")
	runGit(t, tmpDir, "commit", "-m", "initial")

	// Act
	files, err := gitChangedFiles(tmpDir, "nonexistent-ref-xyz")

	// Assert
	require.Error(t, err)
	assert.Nil(t, files)
	assert.ErrorContains(t, err, "nonexistent-ref-xyz")
}

func TestGitChangedFiles_EmptyDiff_ReturnsEmptyMap(t *testing.T) {
	t.Parallel()

	// Arrange
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)
	writeGitFile(t, tmpDir, "a.go", "package a")
	runGit(t, tmpDir, "add", "a.go")
	runGit(t, tmpDir, "commit", "-m", "initial")
	runGit(t, tmpDir, "commit", "--allow-empty", "-m", "empty")

	// Act
	files, err := gitChangedFiles(tmpDir, "HEAD~1")

	// Assert
	require.NoError(t, err)
	assert.Empty(t, files)
}

const testProjectRoot = "/project"

func TestFilterByChangedFiles_KeepsOnlyChangedFiles(t *testing.T) {
	t.Parallel()

	// Arrange
	root := testProjectRoot
	results := []searchResult{
		{directoryPath: "/project", fileName: "a.go"},
		{directoryPath: "/project", fileName: "b.go"},
		{directoryPath: "/project/internal", fileName: "c.go"},
	}
	changed := map[string]bool{
		"a.go":          true,
		"internal/c.go": true,
	}

	// Act
	filtered := filterByChangedFiles(results, root, changed)

	// Assert
	require.Len(t, filtered, 2)
	names := []string{filtered[0].fileName, filtered[1].fileName}
	assert.Contains(t, names, "a.go")
	assert.Contains(t, names, "c.go")
}

func TestFilterByChangedFiles_NoMatches_ReturnsEmptySlice(t *testing.T) {
	t.Parallel()

	// Arrange
	root := testProjectRoot
	results := []searchResult{
		{directoryPath: "/project", fileName: "a.go"},
	}
	changed := map[string]bool{"other.go": true}

	// Act
	filtered := filterByChangedFiles(results, root, changed)

	// Assert
	assert.Empty(t, filtered)
}

func TestFilterByChangedFiles_EmptyResults_ReturnsEmptySlice(t *testing.T) {
	t.Parallel()

	// Arrange
	changed := map[string]bool{"a.go": true}

	// Act
	filtered := filterByChangedFiles([]searchResult{}, testProjectRoot, changed)

	// Assert
	assert.Empty(t, filtered)
}

// initGitRepo sets up a minimal git repo in dir with user config required for commits.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test")
}

func writeGitFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...) // #nosec G204 -- test helper; args come from test literals only
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v failed: %s", args, out)
}
