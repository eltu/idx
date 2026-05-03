package repository

import (
	"fmt"
	"os/exec"

	"go.uber.org/zap"
)

// OSProcessSpawner implements ProcessSpawner using OS commands.
type OSProcessSpawner struct{}

// SpawnWatchProcess starts a watch process in the background.
func (s *OSProcessSpawner) SpawnWatchProcess(projectPath string) (int, error) {
	logger := zap.L()
	logger.Info("spawning watch process", zap.String("projectPath", projectPath))

	cmd := exec.Command("idx", "watch", "--debounce", "1ms")
	cmd.Dir = projectPath
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
