package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"idx/internal/core/domain"
)

func loadInspectTransactionLogs() []inspectLogRow {
	projectRoot, err := resolveInspectProjectRoot()
	if err != nil {
		return []inspectLogRow{}
	}

	logFiles, err := discoverInspectTransactionLogFiles(projectRoot)
	if err != nil {
		return []inspectLogRow{}
	}

	rows := make([]inspectLogRow, 0)
	for _, logFile := range logFiles {
		fileRows, readErr := readInspectTransactionLogFile(logFile)
		if readErr != nil {
			continue
		}

		rows = append(rows, fileRows...)
	}

	sortInspectLogsNewestFirst(rows)

	return rows
}

func resolveInspectProjectRoot() (string, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	searchDir := currentDir
	for {
		gitPath := filepath.Join(searchDir, ".git")
		if info, statErr := os.Stat(gitPath); statErr == nil && info.IsDir() {
			return searchDir, nil
		}

		parentDir := filepath.Dir(searchDir)
		if parentDir == searchDir {
			return currentDir, nil
		}

		searchDir = parentDir
	}
}

func discoverInspectTransactionLogFiles(projectRoot string) ([]string, error) {
	paths := make([]string, 0)
	err := filepath.WalkDir(projectRoot, func(path string, directoryEntry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if directoryEntry.IsDir() {
			if directoryEntry.Name() == ".git" {
				return filepath.SkipDir
			}

			return nil
		}

		if directoryEntry.Name() != "tlog.idx" {
			return nil
		}

		if !isInspectTransactionLogPath(path) {
			return nil
		}

		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(paths)
	return paths, nil
}

func isInspectTransactionLogPath(path string) bool {
	relativePath := filepath.ToSlash(path)
	return strings.HasSuffix(relativePath, "/.idx/logs/tlog.idx") || strings.HasSuffix(relativePath, "/idx/logs/tlog.idx")
}

func readInspectTransactionLogFile(filePath string) ([]inspectLogRow, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	rows := make([]inspectLogRow, 0, len(lines))
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		rows = append(rows, inspectBuildLogRow(trimmed, i+1, filePath))
	}

	for left, right := 0, len(rows)-1; left < right; left, right = left+1, right-1 {
		rows[left], rows[right] = rows[right], rows[left]
	}

	return rows, nil
}

func inspectBuildLogRow(line string, position int, filePath string) inspectLogRow {
	row := inspectLogRow{
		indexedAt: "-",
		path:      filepath.Dir(filepath.Dir(filePath)),
		hash:      "-",
		summary:   line,
		jsonRaw:   line,
	}

	jsonRaw := line

	var parsed map[string]any
	if err := json.Unmarshal([]byte(line), &parsed); err == nil {
		if value, ok := parsed["summary"].(string); ok && strings.TrimSpace(value) != "" {
			row.summary = value
		}

		row.indexedAt = inspectStringField(parsed, "indexed_at", "indexedAt", "timestamp", "time", "updated")
		if row.indexedAt == "" {
			row.indexedAt = "-"
		}

		parsedPath := inspectStringField(parsed, "path", "file_path", "filePath", "directory")
		if parsedPath != "" {
			row.path = parsedPath
		}

		row.hash = inspectStringField(parsed, "hash", "checksum", "sha", "sha256")
		if row.hash == "" {
			row.hash = "-"
		}

		pretty, marshalErr := json.MarshalIndent(parsed, "", "  ")
		if marshalErr == nil {
			jsonRaw = string(pretty)
		}
	}

	indexedAt, pathValue, hash := parseInspectSummaryFields(row.summary)
	if indexedAt != "" {
		row.indexedAt = indexedAt
	}
	if pathValue != "" {
		row.path = pathValue
	}
	if hash != "" {
		row.hash = hash
	}

	row.jsonRaw = jsonRaw
	_ = position
	return row
}

func inspectRowsFromIndex(index *domain.InvertedIndex) ([]inspectDirectoryRow, map[string][]inspectDocumentRow) {
	if index == nil {
		return []inspectDirectoryRow{}, map[string][]inspectDocumentRow{}
	}

	byDirectory := make(map[string][]inspectDocumentRow)
	for documentName, stats := range index.Documents {
		if stats == nil {
			continue
		}

		displayName := stats.Name
		if displayName == "" {
			displayName = documentName
		}

		directory := inspectDocumentDirectory(documentName, stats.Path)
		byDirectory[directory] = append(byDirectory[directory], inspectDocumentRow{
			key:       documentName,
			name:      displayName,
			directory: directory,
			path:      stats.Path,
			length:    stats.Length,
			termCount: documentTermCount(index, documentName),
		})
	}

	directories := make([]inspectDirectoryRow, 0, len(byDirectory))
	for directoryPath, rows := range byDirectory {
		sort.Slice(rows, func(i int, j int) bool {
			if rows[i].path == rows[j].path {
				return rows[i].name < rows[j].name
			}

			return rows[i].path < rows[j].path
		})

		directories = append(directories, inspectDirectoryRow{
			path:          directoryPath,
			documentCount: len(rows),
		})
	}

	sort.Slice(directories, func(i int, j int) bool {
		return directories[i].path < directories[j].path
	})

	return directories, byDirectory
}

func inspectDocumentDirectory(documentKey string, documentPath string) string {
	const separator = "::"
	if separatorIndex := strings.Index(documentKey, separator); separatorIndex > 0 {
		return documentKey[:separatorIndex]
	}

	lastSlash := strings.LastIndex(documentPath, "/")
	lastBackslash := strings.LastIndex(documentPath, "\\")
	separatorIndex := lastSlash
	if lastBackslash > separatorIndex {
		separatorIndex = lastBackslash
	}

	if separatorIndex <= 0 {
		if documentPath != "" {
			return documentPath
		}

		return "."
	}

	return documentPath[:separatorIndex]
}

func documentTermCount(index *domain.InvertedIndex, documentName string) int {
	count := 0
	for _, termStats := range index.Terms {
		if termStats == nil {
			continue
		}

		if _, exists := termStats.Docs[documentName]; exists {
			count++
		}
	}

	return count
}
