package cli

import (
	"fmt"
	"os"

	"idx/internal/shared/filesystem"
)

// initRunner is the subset of indexing.InitCommandService used by InitCommandAdapter.
type initRunner interface {
	Run() error
}

// InitCommandAdapter wraps InitCommandService to satisfy indexing.PathRunner
// by chdir-ing into the target path before delegating to the service.
type InitCommandAdapter struct {
	initService initRunner
	projectTree filesystem.ProjectTree
}

// NewInitCommandAdapter constructs an adapter implementing indexing.PathRunner.
// Example: adapter := NewInitCommandAdapter(initService, projectTree).
func NewInitCommandAdapter(initService initRunner, projectTree filesystem.ProjectTree) *InitCommandAdapter {
	return &InitCommandAdapter{initService: initService, projectTree: projectTree}
}

func (a *InitCommandAdapter) RunFromPath(projectPath string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}
	defer func() { _ = os.Chdir(cwd) }()

	if err := os.Chdir(projectPath); err != nil {
		return fmt.Errorf("failed to change to project directory %q: %w", projectPath, err)
	}

	return a.initService.Run()
}
