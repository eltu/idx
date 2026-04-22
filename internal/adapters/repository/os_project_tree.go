package repository

import (
	"fmt"
	"os"
	"path/filepath"

	"idx/internal/core/domain"
)

type OSProjectTree struct{}

// NewOSProjectTree builds the filesystem adapter used by init.
// Example: projectTree := NewOSProjectTree()
func NewOSProjectTree() OSProjectTree {
	return OSProjectTree{}
}

func (tree OSProjectTree) CurrentDir() (string, error) {
	directoryPath, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to resolve current directory: got error %v, expected a readable working directory", err)
	}

	return directoryPath, nil
}

func (tree OSProjectTree) FindGitRoot(startDir string) (string, error) {
	currentDir := startDir

	for {
		if hasGitEntry(currentDir) {
			return currentDir, nil
		}

		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			break
		}

		currentDir = parentDir
	}

	return "", fmt.Errorf("directory %q is not inside a git project: expected a path with a .git entry in the current directory or one of its parents", startDir)
}

func (tree OSProjectTree) ReadDir(path string) ([]domain.DirectoryEntry, error) {
	directoryEntries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %q: got error %v, expected a readable directory", path, err)
	}

	entries := make([]domain.DirectoryEntry, 0, len(directoryEntries))
	for _, entry := range directoryEntries {
		entries = append(entries, domain.DirectoryEntry{
			Name:  entry.Name(),
			Path:  filepath.Join(path, entry.Name()),
			IsDir: entry.IsDir(),
		})
	}

	return entries, nil
}

func (tree OSProjectTree) Exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}

	if os.IsNotExist(err) {
		return false, nil
	}

	return false, fmt.Errorf("failed to check path %q existence: got error %v, expected a readable filesystem", path, err)
}

func (tree OSProjectTree) WriteFile(path string, content []byte) error {
	if err := os.WriteFile(path, content, 0644); err != nil {
		return fmt.Errorf("failed to write file %q: got error %v, expected a writable path", path, err)
	}

	return nil
}

func hasGitEntry(directoryPath string) bool {
	_, err := os.Stat(filepath.Join(directoryPath, ".git"))
	return err == nil
}
