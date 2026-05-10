package cli

import (
	"fmt"
	"os"

	"idx/internal/core/ports"
	"idx/internal/core/services/indexing"
)

// InitCommandAdapter wraps InitCommandService to satisfy ports.InitCommandInterface
// by chdir-ing into the target path before delegating to the service.
type InitCommandAdapter struct {
	initService indexing.InitCommandService
	projectTree ports.ProjectTree
}

// NewInitCommandAdapter constructs an adapter implementing ports.InitCommandInterface.
// Example: adapter := NewInitCommandAdapter(initService, projectTree).
func NewInitCommandAdapter(initService indexing.InitCommandService, projectTree ports.ProjectTree) *InitCommandAdapter {
	return &InitCommandAdapter{initService: initService, projectTree: projectTree}
}

func (a *InitCommandAdapter) RunFromPath(projectPath string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}
	defer os.Chdir(cwd)

	if err := os.Chdir(projectPath); err != nil {
		return fmt.Errorf("failed to change to project directory %q: %w", projectPath, err)
	}

	return a.initService.Run()
}
