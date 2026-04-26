package repository

import (
	"path/filepath"
	"testing"
)

func TestOSFileReaderReadFileReturnsContent(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "a.txt")
	content := []byte("alpha\nbeta")

	tree := NewOSProjectTree()
	if err := tree.WriteFile(filePath, content); err != nil {
		t.Fatalf("expected file write to succeed, got %v", err)
	}

	reader := NewOSFileReader()
	loaded, err := reader.ReadFile(filePath)
	if err != nil {
		t.Fatalf("expected read to succeed, got %v", err)
	}

	if loaded != string(content) {
		t.Fatalf("expected %q, got %q", string(content), loaded)
	}
}

func TestOSFileReaderReadFileReturnsErrorForMissingFile(t *testing.T) {
	reader := NewOSFileReader()
	_, err := reader.ReadFile(filepath.Join(t.TempDir(), "missing.txt"))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
