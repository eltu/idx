package indexing

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	indexLogMaxSizeBytes    = 1024 * 1024
	indexLogMaxRotatedFiles = 5
)

type indexedFileLogEntry struct {
	Path      string
	Checksum  string
	IndexedAt time.Time
}

func appendIndexedFilesLog(directoryPath string, entries []indexedFileLogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	if !directoryExists(directoryPath) {
		return nil
	}

	logsDir := filepath.Join(directoryPath, ".idx", "logs")
	if err := os.MkdirAll(logsDir, 0750); err != nil {
		return fmt.Errorf("failed to create index logs directory %q: got error %v, expected writable path", logsDir, err)
	}

	activePath := filepath.Join(logsDir, "tlog.idx")
	payload := renderLogEntries(entries)
	if shouldRotateActiveLog(activePath, len(payload)) {
		if err := rotateActiveLogFile(logsDir, activePath); err != nil {
			return err
		}
	}

	if err := appendPayload(activePath, payload); err != nil {
		return err
	}

	return nil
}

func directoryExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return info.IsDir()
}

func renderLogEntries(entries []indexedFileLogEntry) []byte {
	var builder strings.Builder
	for _, entry := range entries {
		builder.WriteString("path=")
		builder.WriteString(entry.Path)
		builder.WriteString("\thash=")
		builder.WriteString(entry.Checksum)
		builder.WriteString("\tindexed_at=")
		builder.WriteString(entry.IndexedAt.UTC().Format(time.RFC3339))
		builder.WriteString("\n")
	}

	return []byte(builder.String())
}

func shouldRotateActiveLog(activePath string, incomingSize int) bool {
	info, err := os.Stat(activePath)
	if err != nil {
		return false
	}

	if info.Size() >= indexLogMaxSizeBytes {
		return true
	}

	return info.Size()+int64(incomingSize) > indexLogMaxSizeBytes
}

func rotateActiveLogFile(logsDir, activePath string) error {
	rotatedName := fmt.Sprintf("tlog_%s.log", time.Now().Format("20060102150405"))
	rotatedPath := filepath.Join(logsDir, rotatedName)
	if _, err := os.Stat(rotatedPath); err == nil {
		rotatedPath = filepath.Join(logsDir, fmt.Sprintf("tlog_%s_%d.log", time.Now().Format("20060102150405"), time.Now().UnixNano()))
	}

	if err := os.Rename(activePath, rotatedPath); err != nil {
		return fmt.Errorf("failed to rotate active index log %q to %q: got error %v, expected writable path", activePath, rotatedPath, err)
	}

	if err := cleanupRotatedLogs(logsDir); err != nil {
		return err
	}

	return nil
}

func cleanupRotatedLogs(logsDir string) error {
	files, err := filepath.Glob(filepath.Join(logsDir, "tlog_*.log"))
	if err != nil {
		return fmt.Errorf("failed to list rotated index logs in %q: got error %v, expected readable directory", logsDir, err)
	}

	if len(files) <= indexLogMaxRotatedFiles {
		return nil
	}

	sort.Slice(files, func(left, right int) bool {
		return filepath.Base(files[left]) > filepath.Base(files[right])
	})

	for _, stale := range files[indexLogMaxRotatedFiles:] {
		if err := os.Remove(stale); err != nil {
			return fmt.Errorf("failed to remove stale rotated log %q: got error %v, expected removable file", stale, err)
		}
	}

	return nil
}

func appendPayload(activePath string, payload []byte) error {
	file, err := os.OpenFile(activePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600) //nolint:gosec
	if err != nil {
		return fmt.Errorf("failed to open active index log %q: got error %v, expected writable path", activePath, err)
	}
	defer func() { _ = file.Close() }()

	if _, err := file.Write(payload); err != nil {
		return fmt.Errorf("failed to append to active index log %q: got error %v, expected writable file", activePath, err)
	}

	return nil
}
