package read_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"idx/internal/features/read"
)

func TestOSFileStreamerOpenFileReadsContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(path, []byte("hello\nworld\n"), 0600); err != nil {
		t.Fatalf("expected file write to succeed, got %v", err)
	}

	streamer := read.NewOSFileStreamer()
	rc, err := streamer.OpenFile(path)
	if err != nil {
		t.Fatalf("expected open to succeed, got %v", err)
	}
	defer rc.Close()

	content, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("expected read to succeed, got %v", err)
	}

	if string(content) != "hello\nworld\n" {
		t.Fatalf("unexpected content %q", string(content))
	}
}

func TestOSFileStreamerOpenFileReturnsErrorForMissingFile(t *testing.T) {
	streamer := read.NewOSFileStreamer()
	_, err := streamer.OpenFile(filepath.Join(t.TempDir(), "missing.txt"))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestOSFileStreamerIsDirReturnsTrueForDirectory(t *testing.T) {
	streamer := read.NewOSFileStreamer()
	isDir, err := streamer.IsDir(t.TempDir())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !isDir {
		t.Fatal("expected true for directory, got false")
	}
}

func TestOSFileStreamerIsDirReturnsFalseForFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("x"), 0600); err != nil {
		t.Fatalf("expected file write to succeed, got %v", err)
	}

	streamer := read.NewOSFileStreamer()
	isDir, err := streamer.IsDir(path)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if isDir {
		t.Fatal("expected false for file, got true")
	}
}

func TestOSFileStreamerIsDirReturnsErrorForMissingPath(t *testing.T) {
	streamer := read.NewOSFileStreamer()
	_, err := streamer.IsDir(filepath.Join(t.TempDir(), "ghost"))
	if err == nil {
		t.Fatal("expected error for missing path, got nil")
	}
}
