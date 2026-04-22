package ports

import "idx/internal/core/domain"

type ProjectTree interface {
	CurrentDir() (string, error)
	FindGitRoot(startDir string) (string, error)
	ReadDir(path string) ([]domain.DirectoryEntry, error)
	Exists(path string) (bool, error)
	WriteFile(path string, content []byte) error
}
