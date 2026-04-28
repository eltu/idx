package repository

import (
	"fmt"
	"os/exec"
)

// OSProcessSpawner implementa ProcessSpawner usando os comandos do SO.
type OSProcessSpawner struct{}

// SpawnWatchProcess inicia um processo watch em background.
func (s *OSProcessSpawner) SpawnWatchProcess(projectPath string) (int, error) {
	cmd := exec.Command("idx", "watch", "--debounce", "750ms")
	cmd.Dir = projectPath
	cmd.Stdout = nil
	cmd.Stderr = nil

	// Inicia o processo desacoplado do terminal pai
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start watch process: %v", err)
	}

	return cmd.Process.Pid, nil
}
