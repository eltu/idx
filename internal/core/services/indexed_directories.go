package services

import (
	"fmt"
	"path/filepath"

	"idx/internal/core/domain"
	"idx/internal/core/ports"
)

func indexedDirectories(projectTree ports.ProjectTree, projectRoot string) ([]string, error) {
	directories := make([]string, 0)
	if err := collectIndexedDirectories(projectTree, projectRoot, &directories); err != nil {
		return nil, err
	}

	return directories, nil
}

func eligibleDirectories(projectTree ports.ProjectTree, projectRoot string, matcher ports.IgnoreMatcher) ([]string, error) {
	directories := make([]string, 0)
	if err := collectEligibleDirectories(projectTree, projectRoot, projectRoot, matcher, &directories); err != nil {
		return nil, err
	}

	return directories, nil
}

func collectIndexedDirectories(projectTree ports.ProjectTree, directoryPath string, directories *[]string) error {
	indexPath := filepath.Join(directoryPath, ".idx", "index.idx")
	hasIndex, err := projectTree.Exists(indexPath)
	if err != nil {
		return err
	}

	if hasIndex {
		*directories = append(*directories, directoryPath)
	}

	entries, err := projectTree.ReadDir(directoryPath)
	if err != nil {
		return fmt.Errorf("failed to read directory %q: got error %v, expected a readable directory", directoryPath, err)
	}

	for _, entry := range entries {
		if !entry.IsDir || entry.Name == ".git" || entry.Name == ".idx" {
			continue
		}

		if err := collectIndexedDirectories(projectTree, entry.Path, directories); err != nil {
			return err
		}
	}

	return nil
}

func collectEligibleDirectories(projectTree ports.ProjectTree, directoryPath string, projectRoot string, matcher ports.IgnoreMatcher, directories *[]string) error {
	*directories = append(*directories, directoryPath)

	entries, err := projectTree.ReadDir(directoryPath)
	if err != nil {
		return fmt.Errorf("failed to read directory %q: got error %v, expected a readable directory", directoryPath, err)
	}

	allowedEntries, err := filterEntries(entries, projectRoot, matcher)
	if err != nil {
		return err
	}

	directoriesToVisit := directoryEntries(allowedEntries)
	for _, entry := range directoriesToVisit {
		if err := collectEligibleDirectories(projectTree, entry.Path, projectRoot, matcher, directories); err != nil {
			return err
		}
	}

	return nil
}

func directoryEntries(entries []domain.DirectoryEntry) []domain.DirectoryEntry {
	directories := make([]domain.DirectoryEntry, 0)
	for _, entry := range entries {
		if !entry.IsDir {
			continue
		}

		directories = append(directories, entry)
	}

	return directories
}
