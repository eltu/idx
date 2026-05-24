package skills_test

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"idx/internal/features/skills"
)

func TestCloneRepoReturnsErrorWhenGitNotFound(t *testing.T) {
	installer := skills.NewOSSkillsInstallerWithDeps(
		func(string) (string, error) { return "", errors.New("git not found") },
		nil, nil, nil,
	)

	_, err := installer.CloneRepo(&strings.Builder{})
	if err == nil {
		t.Fatal("expected error when git is not found, got nil")
	}
}

func TestCloneRepoReturnsErrorWhenMkdirTempFails(t *testing.T) {
	installer := skills.NewOSSkillsInstallerWithDeps(
		func(string) (string, error) { return "/usr/bin/git", nil },
		func(string, string) (string, error) { return "", errors.New("disk full") },
		nil, nil,
	)

	_, err := installer.CloneRepo(&strings.Builder{})
	if err == nil {
		t.Fatal("expected error when mkdirTemp fails, got nil")
	}
}

func TestCloneRepoReturnsErrorWhenGitCloneFails(t *testing.T) {
	tmpDir := t.TempDir()
	cleaned := false

	installer := skills.NewOSSkillsInstallerWithDeps(
		func(string) (string, error) { return "/usr/bin/git", nil },
		func(string, string) (string, error) { return tmpDir, nil },
		func(name string, args ...string) *exec.Cmd {
			return exec.Command("/bin/sh", "-c", "exit 1")
		},
		func(string) error { cleaned = true; return nil },
	)

	_, err := installer.CloneRepo(&strings.Builder{})
	if err == nil {
		t.Fatal("expected error when git clone fails, got nil")
	}
	if !cleaned {
		t.Fatal("expected temp dir to be cleaned up after clone failure")
	}
}

func TestCloneRepoReturnsTempDirOnSuccess(t *testing.T) {
	tmpDir := t.TempDir()

	installer := skills.NewOSSkillsInstallerWithDeps(
		func(string) (string, error) { return "/usr/bin/git", nil },
		func(string, string) (string, error) { return tmpDir, nil },
		func(name string, args ...string) *exec.Cmd {
			return exec.Command("/bin/sh", "-c", "exit 0")
		},
		func(string) error { return nil },
	)

	dir, err := installer.CloneRepo(&strings.Builder{})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if dir != tmpDir {
		t.Fatalf("expected %q, got %q", tmpDir, dir)
	}
}

func TestRunInstallScriptReturnsErrorWhenScriptFails(t *testing.T) {
	installer := skills.NewOSSkillsInstallerWithDeps(
		nil, nil,
		func(name string, args ...string) *exec.Cmd {
			return exec.Command("/bin/sh", "-c", "exit 1")
		},
		nil,
	)

	err := installer.RunInstallScript(t.TempDir(), "vscode", &strings.Builder{})
	if err == nil {
		t.Fatal("expected error when install script fails, got nil")
	}
}

func TestRunInstallScriptSucceeds(t *testing.T) {
	installer := skills.NewOSSkillsInstallerWithDeps(
		nil, nil,
		func(name string, args ...string) *exec.Cmd {
			return exec.Command("/bin/sh", "-c", "exit 0")
		},
		nil,
	)

	err := installer.RunInstallScript(t.TempDir(), "vscode", &strings.Builder{})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestCleanupReturnsErrorWhenRemoveAllFails(t *testing.T) {
	installer := skills.NewOSSkillsInstallerWithDeps(
		nil, nil, nil,
		func(string) error { return errors.New("permission denied") },
	)

	err := installer.Cleanup("/some/dir")
	if err == nil {
		t.Fatal("expected error when removeAll fails, got nil")
	}
}

func TestCleanupSucceeds(t *testing.T) {
	installer := skills.NewOSSkillsInstallerWithDeps(
		nil, nil, nil,
		func(string) error { return nil },
	)

	if err := installer.Cleanup("/some/dir"); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}
