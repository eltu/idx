package indexing

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

func (service InitCommandService) Status() error {
	if err := service.validateDependencies(); err != nil {
		return err
	}

	projectRoot, err := service.resolveProjectRoot()
	if err != nil {
		return err
	}

	directories, err := IndexedDirectories(service.projectTree, projectRoot)
	if err != nil {
		return err
	}

	if len(directories) == 0 {
		return fmt.Errorf("no index found under project root %q: run idx init first", projectRoot)
	}

	for _, directoryPath := range directories {
		if err := service.verifyLatestDirectoryLogEntry(directoryPath); err != nil {
			return err
		}
	}

	return service.output.WriteLine("✅ Indices are up to date.")
}

func (service InitCommandService) resolveProjectRoot() (string, error) {
	currentDir, err := service.projectTree.CurrentDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve current directory: got error %v, expected a readable working directory", err)
	}

	projectRoot, err := service.projectTree.FindGitRoot(currentDir)
	if err != nil {
		return "", err
	}

	return projectRoot, nil
}

func (service InitCommandService) verifyLatestDirectoryLogEntry(directoryPath string) error {
	logPath := filepath.Join(directoryPath, ".idx", "logs", "tlog.idx")

	hasLog, err := service.projectTree.Exists(logPath)
	if err != nil {
		return err
	}

	if !hasLog {
		return fmt.Errorf("missing transaction log at %q: expected an indexed directory with .idx/logs/tlog.idx", logPath)
	}

	entry, err := service.latestTransactionLogEntry(logPath)
	if err != nil {
		return err
	}

	fileUpdatedAt, err := service.fileModTime(entry.Path)
	if err != nil {
		return err
	}

	fileMtime := fileUpdatedAt.UTC().Truncate(time.Second)
	indexedAt := entry.IndexedAt.UTC().Truncate(time.Second)
	if fileMtime.After(indexedAt) {
		return fmt.Errorf("stale index record for path %q: file modified at %q is newer than last indexed_at %q", entry.Path, fileUpdatedAt.Format(time.RFC3339), entry.IndexedAt.Format(time.RFC3339))
	}

	return nil
}

func (service InitCommandService) latestTransactionLogEntry(logPath string) (indexedFileLogEntry, error) {
	content, err := service.fileReader.ReadFile(logPath)
	if err != nil {
		return indexedFileLogEntry{}, fmt.Errorf("failed to read transaction log %q: got error %v, expected a readable file", logPath, err)
	}

	lastLine := lastNonEmptyLine(content)
	if lastLine == "" {
		return indexedFileLogEntry{}, fmt.Errorf("empty transaction log at %q: expected at least one entry with path/hash/indexed_at fields", logPath)
	}

	return parseTransactionLogEntry(lastLine, logPath)
}

func lastNonEmptyLine(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ""
	}

	lines := strings.Split(trimmed, "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

func parseTransactionLogEntry(line string, logPath string) (indexedFileLogEntry, error) {
	indexedAtValue, pathValue, hashValue := parseInspectSummaryFields(line)
	if pathValue == "" || indexedAtValue == "" {
		return indexedFileLogEntry{}, fmt.Errorf("invalid transaction log entry %q in %q: expected fields path=<file> hash=<checksum> indexed_at=<RFC3339>", line, logPath)
	}

	indexedAt, ok := parseInspectLogTime(indexedAtValue)
	if !ok {
		return indexedFileLogEntry{}, fmt.Errorf("invalid indexed_at value %q in %q: expected RFC3339 timestamp", indexedAtValue, logPath)
	}

	return indexedFileLogEntry{Path: pathValue, Checksum: hashValue, IndexedAt: indexedAt.UTC()}, nil
}

func (service InitCommandService) fileModTime(path string) (time.Time, error) {
	entries, err := service.projectTree.ReadDir(filepath.Dir(path))
	if err != nil {
		return time.Time{}, err
	}

	fileName := filepath.Base(path)
	for _, entry := range entries {
		if entry.Name == fileName && !entry.IsDir {
			return time.Unix(0, entry.ModTimeUnixNano).UTC(), nil
		}
	}

	return time.Time{}, fmt.Errorf("file listed in transaction log was not found: got path %q, expected an existing file", path)
}
