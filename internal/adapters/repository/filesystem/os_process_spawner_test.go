package filesystem_test

import (
	"errors"
	"fmt"
	"os/exec"
	"testing"

	"idx/internal/adapters/repository/filesystem"
)

func TestSpawnWatchProcessReturnsErrorWhenExecutableFails(t *testing.T) {
	spawner := filesystem.NewOSProcessSpawnerWithDeps(
		func() (string, error) { return "", errors.New("executable not found") },
		exec.Command,
	)

	_, err := spawner.SpawnWatchProcess(t.TempDir())
	if err == nil {
		t.Fatal("expected error when executable resolution fails, got nil")
	}
}

func TestSpawnWatchProcessReturnsErrorWhenCommandFails(t *testing.T) {
	spawner := filesystem.NewOSProcessSpawnerWithDeps(
		func() (string, error) { return "/nonexistent/binary", nil },
		exec.Command,
	)

	_, err := spawner.SpawnWatchProcess(t.TempDir())
	if err == nil {
		t.Fatal("expected error when command start fails, got nil")
	}
}

func TestSpawnWatchProcessReturnsPIDOnSuccess(t *testing.T) {
	spawner := filesystem.NewOSProcessSpawnerWithDeps(
		func() (string, error) { return "/bin/sh", nil },
		func(name string, args ...string) *exec.Cmd {
			// Replace the real watch command with a no-op that exits immediately.
			return exec.Command("/bin/sh", "-c", fmt.Sprintf("cd %q && exit 0", args))
		},
	)

	pid, err := spawner.SpawnWatchProcess(t.TempDir())
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if pid <= 0 {
		t.Fatalf("expected positive PID, got %d", pid)
	}
}
