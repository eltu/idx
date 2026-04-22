package services

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"idx/internal/core/domain"
	"idx/internal/core/ports"
)

type InitCommandService struct {
	projectTree    ports.ProjectTree
	matcherFactory ports.IgnoreMatcherFactory
	output         ports.TextOutput
}

// NewInitCommandService builds the init use case.
// Example: service := NewInitCommandService(projectTree, matcherFactory, output)
func NewInitCommandService(projectTree ports.ProjectTree, matcherFactory ports.IgnoreMatcherFactory, output ports.TextOutput) InitCommandService {
	return InitCommandService{
		projectTree:    projectTree,
		matcherFactory: matcherFactory,
		output:         output,
	}
}

func (service InitCommandService) Run() error {
	currentDir, err := service.projectTree.CurrentDir()
	if err != nil {
		return fmt.Errorf("failed to resolve current directory: got error %v, expected a readable working directory", err)
	}

	currentIndexPath := indexFilePath(currentDir)
	hasIndex, err := service.projectTree.Exists(currentIndexPath)
	if err != nil {
		return err
	}

	if hasIndex {
		return service.output.WriteLine("ℹ️ Este projeto ja possui indice. Voce pode executar idx search.")
	}

	projectRoot, err := service.projectTree.FindGitRoot(currentDir)
	if err != nil {
		return err
	}

	matcher, err := service.matcherFactory.New(projectRoot)
	if err != nil {
		return fmt.Errorf("failed to load ignore rules for %q: got error %v, expected a readable .gitignore configuration", projectRoot, err)
	}

	if err := service.indexDirectory(currentDir, projectRoot, matcher); err != nil {
		return err
	}

	return service.output.WriteLine("✅ Index created. You can now run idx search.")
}

func (service InitCommandService) indexDirectory(directoryPath string, projectRoot string, matcher ports.IgnoreMatcher) error {
	entries, err := service.projectTree.ReadDir(directoryPath)
	if err != nil {
		return fmt.Errorf("failed to read directory %q: got error %v, expected a readable directory", directoryPath, err)
	}

	allowedEntries, err := filterEntries(entries, projectRoot, matcher)
	if err != nil {
		return err
	}

	if err := service.writeIndex(directoryPath, allowedEntries); err != nil {
		return err
	}

	return service.indexChildren(allowedEntries, projectRoot, matcher)
}

func (service InitCommandService) writeIndex(directoryPath string, entries []domain.DirectoryEntry) error {
	indexPath := indexFilePath(directoryPath)
	content := buildIndexContent(entries)

	if err := service.projectTree.WriteFile(indexPath, []byte(content)); err != nil {
		return fmt.Errorf("failed to write index file %q: got error %v, expected a writable path", indexPath, err)
	}

	return nil
}

func (service InitCommandService) indexChildren(entries []domain.DirectoryEntry, projectRoot string, matcher ports.IgnoreMatcher) error {
	for _, entry := range entries {
		if !entry.IsDir {
			continue
		}

		if err := service.indexDirectory(entry.Path, projectRoot, matcher); err != nil {
			return err
		}
	}

	return nil
}

func filterEntries(entries []domain.DirectoryEntry, projectRoot string, matcher ports.IgnoreMatcher) ([]domain.DirectoryEntry, error) {
	allowedEntries := make([]domain.DirectoryEntry, 0, len(entries))

	for _, entry := range entries {
		if entry.Name == ".git" || entry.Name == ".idx" {
			continue
		}

		relativePath, err := filepath.Rel(projectRoot, entry.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve relative path for %q from %q: got error %v, expected a descendant path", entry.Path, projectRoot, err)
		}

		ignored, err := matcher.Matches(matchPath(relativePath, entry.IsDir))
		if err != nil {
			return nil, err
		}

		if ignored {
			continue
		}

		allowedEntries = append(allowedEntries, entry)
	}

	sort.Slice(allowedEntries, func(left int, right int) bool {
		return allowedEntries[left].Name < allowedEntries[right].Name
	})

	return allowedEntries, nil
}

func matchPath(relativePath string, isDir bool) string {
	normalizedPath := filepath.ToSlash(relativePath)
	if isDir {
		return normalizedPath + "/"
	}

	return normalizedPath
}

func buildIndexContent(entries []domain.DirectoryEntry) string {
	if len(entries) == 0 {
		return ""
	}

	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		lines = append(lines, indexLine(entry))
	}

	return strings.Join(lines, "\n") + "\n"
}

func indexLine(entry domain.DirectoryEntry) string {
	if entry.IsDir {
		return entry.Name + "/"
	}

	return entry.Name
}

func indexFilePath(directoryPath string) string {
	return filepath.Join(directoryPath, ".idx", "index.idx")
}
