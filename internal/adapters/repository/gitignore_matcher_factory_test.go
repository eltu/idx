package repository

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGitIgnoreMatcherMatchesTrackedPathWithNoIndex(t *testing.T) {
	projectRoot := t.TempDir()

	runGitCommand(t, projectRoot, "init")
	runGitCommand(t, projectRoot, "config", "user.email", "idx@example.com")
	runGitCommand(t, projectRoot, "config", "user.name", "idx-test")

	gitignorePath := filepath.Join(projectRoot, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte("internal/\n"), 0600); err != nil {
		t.Fatalf("expected to write .gitignore, got %v", err)
	}

	internalDir := filepath.Join(projectRoot, "internal")
	if err := os.MkdirAll(internalDir, 0750); err != nil {
		t.Fatalf("expected to create internal directory, got %v", err)
	}

	trackedFilePath := filepath.Join(internalDir, "tracked.go")
	if err := os.WriteFile(trackedFilePath, []byte("package internal\n"), 0600); err != nil {
		t.Fatalf("expected to write tracked file, got %v", err)
	}

	runGitCommand(t, projectRoot, "add", ".gitignore")
	runGitCommand(t, projectRoot, "commit", "-m", "add gitignore")
	runGitCommand(t, projectRoot, "add", "-f", "internal/tracked.go")
	runGitCommand(t, projectRoot, "commit", "-m", "add tracked file")

	factory := NewGitIgnoreMatcherFactory()
	matcher, err := factory.New(projectRoot)
	if err != nil {
		t.Fatalf("expected matcher creation to succeed, got %v", err)
	}

	matched, err := matcher.Matches("internal/")
	if err != nil {
		t.Fatalf("expected matcher evaluation to succeed, got %v", err)
	}

	if !matched {
		t.Fatal("expected internal/ to be matched as ignored even when tracked")
	}

	matchedFile, err := matcher.Matches("internal/tracked.go")
	if err != nil {
		t.Fatalf("expected tracked file matcher evaluation to succeed, got %v", err)
	}

	if !matchedFile {
		t.Fatal("expected internal/tracked.go to be matched as ignored even when tracked")
	}
}

func TestGitIgnoreMatcherMatchesReturnsFalseForNonIgnoredPath(t *testing.T) {
	projectRoot := t.TempDir()

	runGitCommand(t, projectRoot, "init")
	runGitCommand(t, projectRoot, "config", "user.email", "idx@example.com")
	runGitCommand(t, projectRoot, "config", "user.name", "idx-test")

	if err := os.WriteFile(filepath.Join(projectRoot, "main.go"), []byte("package main\n"), 0600); err != nil {
		t.Fatalf("expected to create main.go, got %v", err)
	}

	factory := NewGitIgnoreMatcherFactory()
	matcher, err := factory.New(projectRoot)
	if err != nil {
		t.Fatalf("expected matcher creation to succeed, got %v", err)
	}

	matched, err := matcher.Matches("main.go")
	if err != nil {
		t.Fatalf("expected matcher evaluation to succeed, got %v", err)
	}

	if matched {
		t.Fatal("expected main.go to not match ignore rules")
	}
}

func TestGitIgnoreMatcherMatchesReturnsErrorWhenGitFails(t *testing.T) {
	matcher := gitIgnoreMatcher{projectRoot: filepath.Join(t.TempDir(), "missing-project")}

	matched, err := matcher.Matches("main.go")
	if err == nil {
		t.Fatal("expected git check-ignore failure, got nil")
	}

	if matched {
		t.Fatal("expected matched to be false when command fails ")
	}
}

func TestGitIgnoreMatcherFactoryNewReturnsErrorForNonGitDirectory(t *testing.T) {
	factory := NewGitIgnoreMatcherFactory()
	_, err := factory.New(t.TempDir())
	if err == nil {
		t.Fatal("expected matcher factory to fail outside git repository")
	}
}

func runGitCommand(t *testing.T, directory string, args ...string) {
	t.Helper()

	command := exec.Command("git", args...) //nolint:gosec
	command.Dir = directory
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("expected git command %v to succeed, got %v with output %s", args, err, string(output))
	}
}
