package lifecycle

import (
	"errors"
	"fmt"
	"idx/internal/shared/filesystem"
	"idx/internal/shared/output"
	"path/filepath"
)

type DestroyCommandService struct {
	projectTree filesystem.ProjectTree
	output      output.Writer
}

// NewDestroyCommandService builds the destroy use case.
// Example: service := NewDestroyCommandService(projectTree, output).
func NewDestroyCommandService(projectTree filesystem.ProjectTree, output output.Writer) DestroyCommandService {
	return DestroyCommandService{
		projectTree: projectTree,
		output:      output,
	}
}

func (service DestroyCommandService) Run() error {
	if err := service.validateDependencies(); err != nil {
		return err
	}

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

func (service DestroyCommandService) validateDependencies() error {
	if service.projectTree == nil {
		return fmt.Errorf("failed to run destroy command: got nil projectTree dependency, expected non-nil filesystem.ProjectTree")
	}

	if service.output == nil {
		return fmt.Errorf("failed to run destroy command: got nil output dependency, expected non-nil output.Writer")
	}

	return nil
}

func (service DestroyCommandService) destroyIndexes(directoryPath string) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}
	entries, err := service.projectTree.ReadDir(directoryPath)
	if err != nil {
		return fmt.Errorf("failed to read directory %q: got error %v, expected a readable directory", directoryPath, err)
	}
	errs := make([]error, 0)
	for _, entry := range entries {
		if err := service.processDestroyEntry(entry); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (service DestroyCommandService) processDestroyEntry(entry filesystem.DirectoryEntry) error {
	if entry.Name == ".git" || !entry.IsDir {
		return nil
	}
	if entry.Name == ".idx" {
		if err := service.projectTree.RemoveAll(entry.Path); err != nil {
			return fmt.Errorf("failed to remove index directory %q: got error %v, expected a removable .idx directory", entry.Path, err)
		}
		return nil
	}
	return service.destroyIndexes(entry.Path)
}

func isProjectRoot(currentDir, projectRoot string) bool {
	return filepath.Clean(currentDir) == filepath.Clean(projectRoot)
}
