package indexing

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---- appendIndexedFilesLog ----

func TestAppendIndexedFilesLogEmptyEntriesIsNoop(t *testing.T) {
	dir := t.TempDir()
	if err := appendIndexedFilesLog(dir, nil); err != nil {
		t.Fatalf("unexpected error for empty entries: %v", err)
	}
}

func TestAppendIndexedFilesLogNonExistentDirIsNoop(t *testing.T) {
	if err := appendIndexedFilesLog("/nonexistent/xyz/abc", []indexedFileLogEntry{{Path: "x"}}); err != nil {
		t.Fatalf("unexpected error for non-existent dir: %v", err)
	}
}

func TestAppendIndexedFilesLogCreatesLogFile(t *testing.T) {
	dir := t.TempDir()
	entries := []indexedFileLogEntry{
		{Path: "main.go", Checksum: "abc123", IndexedAt: time.Now()},
	}
	if err := appendIndexedFilesLog(dir, entries); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	logPath := filepath.Join(dir, ".idx", "logs", "tlog.idx")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("expected log file to exist: %v", err)
	}
	if !strings.Contains(string(content), "main.go") {
		t.Fatalf("expected log to contain path, got %q", string(content))
	}
}

func TestAppendIndexedFilesLogAppendsOnMultipleCalls(t *testing.T) {
	dir := t.TempDir()
	entries1 := []indexedFileLogEntry{{Path: "a.go", Checksum: "h1", IndexedAt: time.Now()}}
	entries2 := []indexedFileLogEntry{{Path: "b.go", Checksum: "h2", IndexedAt: time.Now()}}
	if err := appendIndexedFilesLog(dir, entries1); err != nil {
		t.Fatalf("unexpected error on first append: %v", err)
	}
	if err := appendIndexedFilesLog(dir, entries2); err != nil {
		t.Fatalf("unexpected error on second append: %v", err)
	}
	logPath := filepath.Join(dir, ".idx", "logs", "tlog.idx")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("expected log file: %v", err)
	}
	if !strings.Contains(string(content), "a.go") || !strings.Contains(string(content), "b.go") {
		t.Fatalf("expected both entries in log, got %q", string(content))
	}
}

// ---- renderLogEntries ----

func TestRenderLogEntriesFormatIsCorrect(t *testing.T) {
	ts := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	entries := []indexedFileLogEntry{{Path: "internal/svc.go", Checksum: "deadbeef", IndexedAt: ts}}
	payload := renderLogEntries(entries)
	line := string(payload)
	if !strings.Contains(line, "path=internal/svc.go") {
		t.Fatalf("expected path in entry, got %q", line)
	}
	if !strings.Contains(line, "hash=deadbeef") {
		t.Fatalf("expected hash in entry, got %q", line)
	}
	if !strings.Contains(line, "2026-01-15") {
		t.Fatalf("expected date in entry, got %q", line)
	}
}

func TestRenderLogEntriesMultipleEntriesHaveNewlines(t *testing.T) {
	entries := []indexedFileLogEntry{
		{Path: "a.go", Checksum: "h1", IndexedAt: time.Now()},
		{Path: "b.go", Checksum: "h2", IndexedAt: time.Now()},
	}
	payload := renderLogEntries(entries)
	lines := strings.Split(strings.TrimRight(string(payload), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), string(payload))
	}
}

// ---- directoryExists ----

func TestDirectoryExistsReturnsTrueForDir(t *testing.T) {
	if !directoryExists(t.TempDir()) {
		t.Fatal("expected true for existing directory")
	}
}

func TestDirectoryExistsReturnsFalseForMissing(t *testing.T) {
	if directoryExists("/nonexistent/xyz") {
		t.Fatal("expected false for missing path")
	}
}

func TestDirectoryExistsReturnsFalseForFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(f, []byte("x"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if directoryExists(f) {
		t.Fatal("expected false for file path")
	}
}

// ---- shouldRotateActiveLog ----

func TestShouldRotateActiveLogReturnsFalseWhenFileMissing(t *testing.T) {
	if shouldRotateActiveLog("/nonexistent/tlog.idx", 100) {
		t.Fatal("expected false when file does not exist")
	}
}

func TestShouldRotateActiveLogReturnsFalseWhenSmall(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tlog.idx")
	if err := os.WriteFile(path, []byte("small"), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if shouldRotateActiveLog(path, 10) {
		t.Fatal("expected false for small file with small incoming payload")
	}
}

func TestShouldRotateActiveLogReturnsTrueWhenAtMaxSize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tlog.idx")
	content := make([]byte, indexLogMaxSizeBytes)
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if !shouldRotateActiveLog(path, 1) {
		t.Fatal("expected true when file is at max size")
	}
}

func TestShouldRotateActiveLogReturnsTrueWhenWouldExceedMax(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tlog.idx")
	content := make([]byte, indexLogMaxSizeBytes-10)
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if !shouldRotateActiveLog(path, 100) {
		t.Fatal("expected true when combined size would exceed max")
	}
}

// ---- rotateActiveLogFile ----

func TestRotateActiveLogFileRenamesFile(t *testing.T) {
	logsDir := t.TempDir()
	activePath := filepath.Join(logsDir, "tlog.idx")
	if err := os.WriteFile(activePath, []byte("log data"), 0600); err != nil {
		t.Fatalf("failed to write active log: %v", err)
	}
	if err := rotateActiveLogFile(logsDir, activePath); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(activePath); err == nil {
		t.Fatal("expected active log to be renamed away")
	}
	files, _ := filepath.Glob(filepath.Join(logsDir, "tlog_*.log"))
	if len(files) == 0 {
		t.Fatal("expected rotated log file to exist")
	}
}

func TestRotateActiveLogFileReturnsErrorWhenRenameFails(t *testing.T) {
	logsDir := t.TempDir()
	activePath := filepath.Join(logsDir, "nonexistent.idx")
	err := rotateActiveLogFile(logsDir, activePath)
	if err == nil {
		t.Fatal("expected error when active log does not exist")
	}
}

// ---- cleanupRotatedLogs ----

func TestCleanupRotatedLogsRemovesOldestWhenOverLimit(t *testing.T) {
	logsDir := t.TempDir()
	for i := range indexLogMaxRotatedFiles + 3 {
		name := fmt.Sprintf("tlog_2026010%d120000.log", i)
		if err := os.WriteFile(filepath.Join(logsDir, name), []byte("data"), 0600); err != nil {
			t.Fatalf("failed to create log file: %v", err)
		}
	}
	if err := cleanupRotatedLogs(logsDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	remaining, _ := filepath.Glob(filepath.Join(logsDir, "tlog_*.log"))
	if len(remaining) != indexLogMaxRotatedFiles {
		t.Fatalf("expected %d remaining logs, got %d", indexLogMaxRotatedFiles, len(remaining))
	}
}

func TestCleanupRotatedLogsDoesNothingWhenUnderLimit(t *testing.T) {
	logsDir := t.TempDir()
	for i := range 3 {
		name := fmt.Sprintf("tlog_202601%02d120000.log", i+1)
		if err := os.WriteFile(filepath.Join(logsDir, name), []byte("data"), 0600); err != nil {
			t.Fatalf("failed to create log file: %v", err)
		}
	}
	if err := cleanupRotatedLogs(logsDir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	remaining, _ := filepath.Glob(filepath.Join(logsDir, "tlog_*.log"))
	if len(remaining) != 3 {
		t.Fatalf("expected 3 logs to remain, got %d", len(remaining))
	}
}

// ---- appendIndexedFilesLog MkdirAll error ----

func TestAppendIndexedFilesLogMkdirAllErrorReturnsError(t *testing.T) {
	// Create a file at the .idx path so MkdirAll(".idx/logs") fails.
	dir := t.TempDir()
	idxPath := filepath.Join(dir, ".idx")
	if err := os.WriteFile(idxPath, []byte("not a directory"), 0600); err != nil {
		t.Fatalf("failed to create .idx file: %v", err)
	}
	entries := []indexedFileLogEntry{{Path: "main.go", Checksum: "abc", IndexedAt: time.Now()}}
	err := appendIndexedFilesLog(dir, entries)
	if err == nil {
		t.Fatal("expected error when MkdirAll fails (file in place of directory)")
	}
}

// ---- appendIndexedFilesLog with rotation ----

func TestAppendIndexedFilesLogRotatesWhenAtMaxSize(t *testing.T) {
	dir := t.TempDir()
	logsDir := filepath.Join(dir, ".idx", "logs")
	if err := os.MkdirAll(logsDir, 0750); err != nil {
		t.Fatalf("failed to create logs dir: %v", err)
	}
	activePath := filepath.Join(logsDir, "tlog.idx")
	oversized := make([]byte, indexLogMaxSizeBytes)
	if err := os.WriteFile(activePath, oversized, 0600); err != nil {
		t.Fatalf("failed to write oversized log: %v", err)
	}
	entries := []indexedFileLogEntry{{Path: "main.go", Checksum: "abc", IndexedAt: time.Now()}}
	if err := appendIndexedFilesLog(dir, entries); err != nil {
		t.Fatalf("unexpected error during rotation: %v", err)
	}
	rotated, _ := filepath.Glob(filepath.Join(logsDir, "tlog_*.log"))
	if len(rotated) == 0 {
		t.Fatal("expected a rotated log file after max-size rotation")
	}
}
