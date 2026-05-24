package storage

import (
	"path/filepath"
	"testing"
)

func TestIndexFilePathBuildsStandardPath(t *testing.T) {
	directoryPath := filepath.Join("repo", "pkg")
	expected := filepath.Join(directoryPath, ".idx", "index.idx")

	if indexFilePath(directoryPath) != expected {
		t.Fatalf("expected %q, got %q", expected, indexFilePath(directoryPath))
	}
}
