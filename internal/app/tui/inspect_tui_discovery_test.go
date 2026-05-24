package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverInspectTransactionLogFilesFindsAllDirectories(t *testing.T) {
	root := t.TempDir()

	paths := []string{
		filepath.Join(root, ".idx", "logs", "tlog.idx"),
		filepath.Join(root, "internal", ".idx", "logs", "tlog.idx"),
		filepath.Join(root, "cmd", "idx", "logs", "tlog.idx"),
	}
	for _, path := range paths {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("expected directory creation without error, got %v", err)
		}
		if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
			t.Fatalf("expected file creation without error, got %v", err)
		}
	}

	found, err := discoverInspectTransactionLogFiles(root)
	if err != nil {
		t.Fatalf("expected discovery without error, got %v", err)
	}

	if len(found) != 3 {
		t.Fatalf("expected 3 tlog files, got %d", len(found))
	}
}
