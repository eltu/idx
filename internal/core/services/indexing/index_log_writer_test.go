package indexing

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAppendIndexedFilesLogWritesActiveLog(t *testing.T) {
	directory := t.TempDir()
	entries := []indexedFileLogEntry{
		{Path: filepath.Join(directory, "a.txt"), Checksum: "abc123", IndexedAt: time.Date(2026, 4, 26, 14, 22, 30, 0, time.UTC)},
		{Path: filepath.Join(directory, "b.txt"), Checksum: "def456", IndexedAt: time.Date(2026, 4, 26, 14, 22, 31, 0, time.UTC)},
	}

	if err := appendIndexedFilesLog(directory, entries); err != nil {
		t.Fatalf("expected log append to succeed, got %v", err)
	}

	activePath := filepath.Join(directory, ".idx", "logs", "tlog.idx")
	content, err := os.ReadFile(activePath) //nolint:gosec
	if err != nil {
		t.Fatalf("expected active log to be readable, got %v", err)
	}

	text := string(content)
	line1 := fmt.Sprintf("path=%s\thash=%s\tindexed_at=%s", entries[0].Path, entries[0].Checksum, entries[0].IndexedAt.Format(time.RFC3339))
	line2 := fmt.Sprintf("path=%s\thash=%s\tindexed_at=%s", entries[1].Path, entries[1].Checksum, entries[1].IndexedAt.Format(time.RFC3339))
	if !strings.Contains(text, line1) {
		t.Fatalf("expected active log to contain standardized first line %q, got %q", line1, text)
	}
	if !strings.Contains(text, line2) {
		t.Fatalf("expected active log to contain standardized second line %q, got %q", line2, text)
	}
}

func TestAppendIndexedFilesLogRotatesAtOneMegabyte(t *testing.T) {
	directory := t.TempDir()
	logsDir := filepath.Join(directory, ".idx", "logs")
	if err := os.MkdirAll(logsDir, 0750); err != nil {
		t.Fatalf("expected logs dir creation to succeed, got %v", err)
	}

	activePath := filepath.Join(logsDir, "tlog.idx")
	if err := os.WriteFile(activePath, []byte(strings.Repeat("x", indexLogMaxSizeBytes)), 0600); err != nil {
		t.Fatalf("expected active log seed write to succeed, got %v", err)
	}

	entry := indexedFileLogEntry{Path: filepath.Join(directory, "a.txt"), Checksum: "abc123", IndexedAt: time.Now().UTC()}
	if err := appendIndexedFilesLog(directory, []indexedFileLogEntry{entry}); err != nil {
		t.Fatalf("expected rotating append to succeed, got %v", err)
	}

	rotated, err := filepath.Glob(filepath.Join(logsDir, "tlog_*.log"))
	if err != nil {
		t.Fatalf("expected glob to succeed, got %v", err)
	}
	if len(rotated) != 1 {
		t.Fatalf("expected one rotated log file, got %d", len(rotated))
	}

	current, err := os.ReadFile(activePath) //nolint:gosec
	if err != nil {
		t.Fatalf("expected active log read to succeed, got %v", err)
	}
	if !strings.Contains(string(current), entry.Checksum) {
		t.Fatalf("expected active log to contain new entry checksum, got %q", string(current))
	}
}

func TestCleanupRotatedLogsKeepsLatestFive(t *testing.T) {
	logsDir := t.TempDir()
	for index := 0; index < 7; index++ {
		name := fmt.Sprintf("tlog_202604261422%02d.log", index)
		if err := os.WriteFile(filepath.Join(logsDir, name), []byte("x"), 0600); err != nil {
			t.Fatalf("expected rotated log seed write to succeed, got %v", err)
		}
	}

	if err := cleanupRotatedLogs(logsDir); err != nil {
		t.Fatalf("expected cleanup to succeed, got %v", err)
	}

	remaining, err := filepath.Glob(filepath.Join(logsDir, "tlog_*.log"))
	if err != nil {
		t.Fatalf("expected glob to succeed, got %v", err)
	}
	if len(remaining) != indexLogMaxRotatedFiles {
		t.Fatalf("expected %d rotated logs, got %d", indexLogMaxRotatedFiles, len(remaining))
	}

	for _, filePath := range remaining {
		name := filepath.Base(filePath)
		if strings.Contains(name, "00.log") || strings.Contains(name, "01.log") {
			t.Fatalf("expected oldest rotated logs removed, still found %q", name)
		}
	}
}
