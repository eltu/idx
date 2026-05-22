package filesystem

import (
	"fmt"
	"os"
	"os/exec"

	"go.uber.org/zap"
)

// OSProcessSpawner implements ProcessSpawner using OS commands.
type OSProcessSpawner struct {
	executableFn   func() (string, error)
	commandBuilder func(name string, args ...string) *exec.Cmd
}

// NewOSProcessSpawner creates a spawner backed by real OS calls.
func NewOSProcessSpawner() *OSProcessSpawner {
	return NewOSProcessSpawnerWithDeps(os.Executable, exec.Command)
}

// NewOSProcessSpawnerWithDeps creates a spawner with injected OS dependencies for testing.
func NewOSProcessSpawnerWithDeps(
	executableFn func() (string, error),
	commandBuilder func(name string, args ...string) *exec.Cmd,
) *OSProcessSpawner {
	return &OSProcessSpawner{
		executableFn:   executableFn,
		commandBuilder: commandBuilder,
	}
}

// SpawnWatchProcess starts a watch process in the background.
func (s *OSProcessSpawner) SpawnWatchProcess(projectPath string) (int, error) {
	logger := zap.L()
	logger.Info("spawning watch process", zap.String("projectPath", projectPath))

	selfPath, err := s.executableFn()
	if err != nil {
		return 0, fmt.Errorf("failed to resolve executable path: %w", err)
	}

	cmd := s.commandBuilder(selfPath, "watch", "--debounce", "1ms")
	cmd.Dir = projectPath
	cmd.Env = append(cmd.Environ(), "IDX_DAEMON_CHILD=1")
	cmd.Stdout = nil
	cmd.Stderr = nil

	// Start process detached from parent terminal.
	if err := cmd.Start(); err != nil {
		logger.Error("failed to spawn watch process", zap.String("projectPath", projectPath), zap.Error(err))
		return 0, fmt.Errorf("failed to start watch process: %v", err)
	}

	logger.Info("watch process started", zap.String("projectPath", projectPath), zap.Int("pid", cmd.Process.Pid))
	return cmd.Process.Pid, nil
}
