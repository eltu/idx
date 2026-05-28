package daemon

import (
	"fmt"
	"os"
	"os/exec"

	"go.uber.org/zap"
)

const (
	serverDaemonEnvVar   = "IDX_SERVER_DAEMON"
	serverProjectPathVar = "IDX_PROJECT_PATH"
)

// OSServerSpawner implements ServerSpawner using OS commands.
type OSServerSpawner struct {
	executableFn   func() (string, error)
	commandBuilder func(name string, args ...string) *exec.Cmd
}

// NewOSServerSpawner creates a ServerSpawner backed by real OS calls.
// Example: NewOSServerSpawner().SpawnServerProcess("/home/user/myproject").
func NewOSServerSpawner() *OSServerSpawner {
	return NewOSServerSpawnerWithDeps(os.Executable, exec.Command)
}

// NewOSServerSpawnerWithDeps creates a spawner with injected OS dependencies for testing.
func NewOSServerSpawnerWithDeps(
	executableFn func() (string, error),
	commandBuilder func(name string, args ...string) *exec.Cmd,
) *OSServerSpawner {
	return &OSServerSpawner{
		executableFn:   executableFn,
		commandBuilder: commandBuilder,
	}
}

// SpawnServerProcess starts `idx server run` in the background with IDX_SERVER_DAEMON=1.
func (s *OSServerSpawner) SpawnServerProcess(projectPath string) (int, error) {
	logger := zap.L()
	logger.Info("spawning server process", zap.String("projectPath", projectPath))

	selfPath, err := s.executableFn()
	if err != nil {
		return 0, fmt.Errorf("failed to resolve executable path: %w", err)
	}

	cmd := s.commandBuilder(selfPath, "server", "run")
	cmd.Dir = projectPath
	cmd.Env = append(cmd.Environ(),
		serverDaemonEnvVar+"=1",
		serverProjectPathVar+"="+projectPath,
	)
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		logger.Error("failed to spawn server process", zap.String("projectPath", projectPath), zap.Error(err))
		return 0, fmt.Errorf("failed to start server process: %v", err)
	}

	logger.Info("server process started", zap.String("projectPath", projectPath), zap.Int("pid", cmd.Process.Pid))
	return cmd.Process.Pid, nil
}
