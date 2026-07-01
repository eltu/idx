package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- helpers ----

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")
}

func writeGitFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("writeGitFile: %v", err)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...) //nolint:gosec
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// ---- resolveGitBinary ----

func TestResolveGitBinary_GitOnPath_ReturnsAbsolutePath(t *testing.T) {
	t.Parallel()

	// Act
	gitPath, err := resolveGitBinary()

	// Assert
	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(gitPath))
}

func TestResolveGitBinary_GitMissingFromPath_ReturnsError(t *testing.T) {
	// Arrange
	t.Setenv("PATH", t.TempDir())

	// Act
	_, err := resolveGitBinary()

	// Assert
	require.Error(t, err)
	assert.ErrorContains(t, err, "git executable not found in PATH")
}

// ---- ChangedFilesSince ----

func TestChangedFilesSince_ReturnsChangedFiles(t *testing.T) {
	t.Parallel()

	// Arrange
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)
	writeGitFile(t, tmpDir, "a.go", "package a")
	runGit(t, tmpDir, "add", ".")
	runGit(t, tmpDir, "commit", "-m", "first")
	writeGitFile(t, tmpDir, "b.go", "package b")
	runGit(t, tmpDir, "add", ".")
	runGit(t, tmpDir, "commit", "-m", "second")

	// Act
	files, err := ChangedFilesSince(tmpDir, "HEAD~1")

	// Assert
	require.NoError(t, err)
	assert.True(t, files["b.go"])
	assert.False(t, files["a.go"])
}

func TestChangedFilesSince_InvalidRef_ReturnsError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)
	writeGitFile(t, tmpDir, "a.go", "package a")
	runGit(t, tmpDir, "add", ".")
	runGit(t, tmpDir, "commit", "-m", "init")

	_, err := ChangedFilesSince(tmpDir, "nonexistent-ref-xyz")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent-ref-xyz")
}

// ---- CoChangedFiles ----

func TestCoChangedFiles_InRealRepo_CountsCorrectly(t *testing.T) {
	t.Parallel()

	// Arrange
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)
	writeGitFile(t, tmpDir, "target.go", "package main")
	writeGitFile(t, tmpDir, "sibling.go", "package main")
	runGit(t, tmpDir, "add", ".")
	runGit(t, tmpDir, "commit", "-m", "first commit")

	// Act
	coChanges, total, err := CoChangedFiles(tmpDir, "target.go")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Equal(t, 1, coChanges["sibling.go"])
	assert.NotContains(t, coChanges, "target.go")
}

func TestCoChangedFiles_NoHistory_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	// Arrange: repo with commits, but target file was never committed.
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)
	writeGitFile(t, tmpDir, "other.go", "package main")
	runGit(t, tmpDir, "add", ".")
	runGit(t, tmpDir, "commit", "-m", "init")

	// Act
	coChanges, total, err := CoChangedFiles(tmpDir, "target.go")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Empty(t, coChanges)
}

// ---- parseCoChangeFiles ----

func TestParseCoChangeFiles_TwoCommits_CountsCorrectly(t *testing.T) {
	t.Parallel()

	// Simulate diff-tree output: SHA header per commit, then files touched.
	sha1 := strings.Repeat("a", 40)
	sha2 := strings.Repeat("b", 40)
	raw := sha1 + "\ntarget.go\nfoo.go\nbar.go\n" + sha2 + "\ntarget.go\nfoo.go\n"
	coChanges, total, err := parseCoChangeFiles(raw, "target.go", 2)

	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Equal(t, 2, coChanges["foo.go"])
	assert.Equal(t, 1, coChanges["bar.go"])
	assert.NotContains(t, coChanges, "target.go")
}

func TestParseCoChangeFiles_EmptyOutput_ZeroCommits(t *testing.T) {
	t.Parallel()

	coChanges, total, err := parseCoChangeFiles("", "target.go", 0)

	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Empty(t, coChanges)
}

// ---- isGitSHA ----

func TestIsGitSHA_ValidSHA_ReturnsTrue(t *testing.T) {
	t.Parallel()

	assert.True(t, isGitSHA(strings.Repeat("a", 40)))
	assert.True(t, isGitSHA("0123456789abcdef0123456789abcdef01234567"))
}

func TestIsGitSHA_WrongLength_ReturnsFalse(t *testing.T) {
	t.Parallel()

	assert.False(t, isGitSHA(strings.Repeat("a", 39)))
	assert.False(t, isGitSHA(strings.Repeat("a", 41)))
	assert.False(t, isGitSHA(""))
}

func TestIsGitSHA_UppercaseHex_ReturnsFalse(t *testing.T) {
	t.Parallel()

	// Git SHA-1 output is always lowercase; uppercase is not a valid SHA.
	assert.False(t, isGitSHA(strings.Repeat("A", 40)))
}

func TestIsGitSHA_NonHexChars_ReturnsFalse(t *testing.T) {
	t.Parallel()

	assert.False(t, isGitSHA(strings.Repeat("g", 40)))
	assert.False(t, isGitSHA("not-a-sha-at-all-in-length-0000000000000"))
}
