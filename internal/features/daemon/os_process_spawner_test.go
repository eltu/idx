package daemon_test

import (
	"errors"
	"os/exec"
	"testing"

	"idx/internal/features/daemon"
)

func TestOSProcessSpawnerExecutableFnError(t *testing.T) {
	spawner := daemon.NewOSProcessSpawnerWithDeps(
		func() (string, error) { return "", errors.New("executable not found") },
		exec.Command,
	)
	_, err := spawner.SpawnWatchProcess(t.TempDir())
	if err == nil {
		t.Fatal("expected error when executableFn fails")
	}
}

func TestOSProcessSpawnerStartFailure(t *testing.T) {
	spawner := daemon.NewOSProcessSpawnerWithDeps(
		func() (string, error) { return "/nonexistent/binary/xyz", nil },
		exec.Command,
	)
	_, err := spawner.SpawnWatchProcess(t.TempDir())
	if err == nil {
		t.Fatal("expected error when command cannot start")
	}
}

func TestOSProcessSpawnerSuccessReturnsPID(t *testing.T) {
	// Use /usr/bin/true as a stand-in self-path; exec.Command will start it
	// with extra args which it ignores and exits 0 immediately.
	spawner := daemon.NewOSProcessSpawnerWithDeps(
		func() (string, error) { return "/usr/bin/true", nil },
		exec.Command,
	)
	pid, err := spawner.SpawnWatchProcess(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pid <= 0 {
		t.Fatalf("expected positive PID, got %d", pid)
	}
}

func TestNewOSProcessSpawnerIsNotNil(t *testing.T) {
	s := daemon.NewOSProcessSpawner()
	if s == nil {
		t.Fatal("expected non-nil spawner")
	}
}
