package filesystem

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitIgnoreMatcher_Matches_IgnoresTrackedPathCoveredByGitignore(t *testing.T) {
	t.Parallel()

	// Arrange
	projectRoot := t.TempDir()
	runGitCommand(t, projectRoot, "init")
	runGitCommand(t, projectRoot, "config", "user.email", "idx@example.com")
	runGitCommand(t, projectRoot, "config", "user.name", "idx-test")

	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, ".gitignore"), []byte("internal/\n"), 0600))

	internalDir := filepath.Join(projectRoot, "internal")
	require.NoError(t, os.MkdirAll(internalDir, 0750))
	require.NoError(t, os.WriteFile(filepath.Join(internalDir, "tracked.go"), []byte("package internal\n"), 0600))

	runGitCommand(t, projectRoot, "add", ".gitignore")
	runGitCommand(t, projectRoot, "commit", "-m", "add gitignore")
	runGitCommand(t, projectRoot, "add", "-f", "internal/tracked.go")
	runGitCommand(t, projectRoot, "commit", "-m", "add tracked file")

	factory := NewGitIgnoreMatcherFactory()
	matcher, err := factory.New(projectRoot)
	require.NoError(t, err)

	// Act & Assert — directory match
	matched, err := matcher.Matches("internal/")
	require.NoError(t, err)
	assert.True(t, matched, "expected internal/ to be matched even when tracked")

	// Act & Assert — file match
	matchedFile, err := matcher.Matches("internal/tracked.go")
	require.NoError(t, err)
	assert.True(t, matchedFile, "expected internal/tracked.go to be matched even when tracked")
}

func TestGitIgnoreMatcher_Matches_ReturnsFalseForNonIgnoredPath(t *testing.T) {
	t.Parallel()

	// Arrange
	projectRoot := t.TempDir()
	runGitCommand(t, projectRoot, "init")
	runGitCommand(t, projectRoot, "config", "user.email", "idx@example.com")
	runGitCommand(t, projectRoot, "config", "user.name", "idx-test")
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "main.go"), []byte("package main\n"), 0600))

	factory := NewGitIgnoreMatcherFactory()
	matcher, err := factory.New(projectRoot)
	require.NoError(t, err)

	// Act
	matched, err := matcher.Matches("main.go")

	// Assert
	require.NoError(t, err)
	assert.False(t, matched)
}

func TestGitIgnoreMatcher_Matches_ReturnsErrorWhenGitFails(t *testing.T) {
	t.Parallel()

	// Arrange
	matcher := gitIgnoreMatcher{projectRoot: filepath.Join(t.TempDir(), "missing-project")}

	// Act
	matched, err := matcher.Matches("main.go")

	// Assert
	require.Error(t, err)
	assert.False(t, matched)
}

func TestGitIgnoreMatcherFactory_New_ReturnsErrorForNonGitDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	factory := NewGitIgnoreMatcherFactory()

	// Act
	_, err := factory.New(t.TempDir())

	// Assert
	require.Error(t, err)
}

func runGitCommand(t *testing.T, directory string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...) //nolint:gosec
	command.Dir = directory
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("expected git command %v to succeed, got %v with output %s", args, err, string(output))
	}
}
