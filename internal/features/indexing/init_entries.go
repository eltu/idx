package indexing

import (
	"fmt"
	"idx/internal/shared/filesystem"
	"path/filepath"
	"sort"
)

func filterEntries(entries []filesystem.DirectoryEntry, projectRoot string, matcher filesystem.IgnoreMatcher) ([]filesystem.DirectoryEntry, error) {
	allowedEntries := make([]filesystem.DirectoryEntry, 0, len(entries))
	for _, entry := range entries {
		allowed, err := isEntryAllowed(entry, projectRoot, matcher)
		if err != nil {
			return nil, err
		}
		if allowed {
			allowedEntries = append(allowedEntries, entry)
		}
	}
	sort.Slice(allowedEntries, func(left, right int) bool {
		return allowedEntries[left].Name < allowedEntries[right].Name
	})
	return allowedEntries, nil
}

func isEntryAllowed(entry filesystem.DirectoryEntry, projectRoot string, matcher filesystem.IgnoreMatcher) (bool, error) {
	if entry.Name == ".git" || entry.Name == ".idx" || entry.IsSymlink {
		return false, nil
	}
	relativePath, err := filepath.Rel(projectRoot, entry.Path)
	if err != nil {
		return false, fmt.Errorf("failed to resolve relative path for %q from %q: got error %v, expected a descendant path", entry.Path, projectRoot, err)
	}
	ignored, err := matcher.Matches(matchPath(relativePath, entry.IsDir))
	if err != nil {
		return false, err
	}
	return !ignored, nil
}

func matchPath(relativePath string, isDir bool) string {
	normalizedPath := filepath.ToSlash(relativePath)
	if isDir {
		return normalizedPath + "/"
	}

	return normalizedPath
}

func filesFromEntries(entries []filesystem.DirectoryEntry) []filesystem.DirectoryEntry {
	fileEntries := make([]filesystem.DirectoryEntry, 0)
	for _, entry := range entries {
		if entry.IsDir {
			continue
		}

		fileEntries = append(fileEntries, entry)
	}

	return fileEntries
}
