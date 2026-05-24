package indexing_test

import (
	"os"
	"path/filepath"
	"testing"

	"idx/internal/features/indexing"
	"idx/internal/features/indexing/storage"
	"idx/internal/shared/filesystem"
)

func TestInitSkipsSymlinkPointingToDirectory(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(rootDir, ".git"), 0o750); err != nil {
		t.Fatalf("expected git marker creation to succeed, got %v", err)
	}

	if err := os.MkdirAll(filepath.Join(rootDir, ".claude", "target"), 0o750); err != nil {
		t.Fatalf("expected target directory creation to succeed, got %v", err)
	}

	symlinkPath := filepath.Join(rootDir, ".claude", "skills")
	if err := os.Symlink(filepath.Join(rootDir, ".claude", "target"), symlinkPath); err != nil {
		t.Fatalf("expected symlink creation to succeed, got %v", err)
	}

	rootFile := filepath.Join(rootDir, "README.txt")
	if err := os.WriteFile(rootFile, []byte("hello"), 0o600); err != nil {
		t.Fatalf("expected root file creation to succeed, got %v", err)
	}

	projectTree := filesystem.NewOSProjectTree()
	service := indexing.NewInitCommandService(indexing.InitCommandServiceDeps{
		ProjectTree:    projectTree,
		MatcherFactory: fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}},
		Output:         &capturingTextOutput{},
		FileReader:     filesystem.NewOSFileReader(),
		Indexer:        indexing.NewBM25IndexService(),
		IndexRepo:      storage.NewBinaryIndexRepository(),
		ChecksumRepo:   storage.NewDirectoryChecksumRepository(),
		DaemonRepo:     nil,
	})

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("expected cwd read to succeed, got %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalDir) })

	if err := os.Chdir(rootDir); err != nil {
		t.Fatalf("expected chdir to test root to succeed, got %v", err)
	}

	if err := service.Run(); err != nil {
		t.Fatalf("expected init to succeed when directory symlink is present, got %v", err)
	}
}
