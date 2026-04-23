package services

import (
	"errors"
	"fmt"
	"path/filepath"

	"idx/internal/core/ports"
)

type DestroyCommandService struct {
	projectTree ports.ProjectTree
	output      ports.TextOutput
}

// NewDestroyCommandService builds the destroy use case.
// Example: service := NewDestroyCommandService(projectTree, output).
func NewDestroyCommandService(projectTree ports.ProjectTree, output ports.TextOutput) DestroyCommandService {
	return DestroyCommandService{
		projectTree: projectTree,
		output:      output,
	}
}

func (service DestroyCommandService) Run() error {
	currentDir, err := service.projectTree.CurrentDir()
	if err != nil {
		return fmt.Errorf("failed to resolve current directory: got error %v, expected a readable working directory", err)
	}

	projectRoot, err := service.projectTree.FindGitRoot(currentDir)
	if err != nil {
		return err
	}

	if !isProjectRoot(currentDir, projectRoot) {
		return fmt.Errorf("destroy must run from project root: got current directory %q, expected root directory %q", currentDir, projectRoot)
	}

	if err := service.destroyIndexes(projectRoot); err != nil {
		return err
	}

	return service.output.WriteLine("🧹 Index metadata removed from project.")
}

func (service DestroyCommandService) destroyIndexes(directoryPath string) error {
	entries, err := service.projectTree.ReadDir(directoryPath)
	if err != nil {
		return fmt.Errorf("failed to read directory %q: got error %v, expected a readable directory", directoryPath, err)
	}

	collectedErrors := make([]error, 0)

	for _, entry := range entries {
		if entry.Name == ".git" {
			continue
		}

		if entry.IsDir && entry.Name == ".idx" {
			if err := service.projectTree.RemoveAll(entry.Path); err != nil {
				collectedErrors = append(collectedErrors, fmt.Errorf("failed to remove index directory %q: got error %v, expected a removable .idx directory", entry.Path, err))
				continue
			}

			continue
		}

		if entry.IsDir {
			if err := service.destroyIndexes(entry.Path); err != nil {
				collectedErrors = append(collectedErrors, err)
			}
		}
	}

	return errors.Join(collectedErrors...)
}

func isProjectRoot(currentDir string, projectRoot string) bool {
	return filepath.Clean(currentDir) == filepath.Clean(projectRoot)
}
