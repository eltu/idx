package indexing_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"idx/internal/features/indexing"
	"idx/internal/features/indexing/storage"
	"idx/internal/shared/filesystem"
)

func TestInitAndSyncAppendIndexLogWhenFilesChange(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(rootDir, ".git"), 0750); err != nil {
		t.Fatalf("expected git marker creation to succeed, got %v", err)
	}

	filePath := filepath.Join(rootDir, "doc.txt")
	if err := os.WriteFile(filePath, []byte("needle v1\n"), 0600); err != nil {
		t.Fatalf("expected seed file creation to succeed, got %v", err)
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
		t.Fatalf("expected init to succeed, got %v", err)
	}

	logPath := filepath.Join(rootDir, ".idx", "logs", "tlog.idx")
	beforeInfo, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("expected init log file to exist, got %v", err)
	}
	beforeSize := beforeInfo.Size()
	if beforeSize <= 0 {
		t.Fatalf("expected init log to have content, got size %d", beforeSize)
	}

	if err := os.WriteFile(filePath, []byte("needle v2\n"), 0600); err != nil {
		t.Fatalf("expected file mutation before sync to succeed, got %v", err)
	}

	if err := service.Sync(); err != nil {
		t.Fatalf("expected sync to succeed, got %v", err)
	}

	afterInfo, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("expected sync log file to exist, got %v", err)
	}
	afterSize := afterInfo.Size()
	if afterSize <= beforeSize {
		t.Fatalf("expected sync to append to log (before=%d after=%d)", beforeSize, afterSize)
	}
}

func TestSyncDoesNotAppendIndexLogWhenFilesUnchanged(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(rootDir, ".git"), 0750); err != nil {
		t.Fatalf("expected git marker creation to succeed, got %v", err)
	}

	filePath := filepath.Join(rootDir, "doc.txt")
	if err := os.WriteFile(filePath, []byte("needle stable\n"), 0600); err != nil {
		t.Fatalf("expected seed file creation to succeed, got %v", err)
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
		t.Fatalf("expected init to succeed, got %v", err)
	}

	logPath := filepath.Join(rootDir, ".idx", "logs", "tlog.idx")
	beforeInfo, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("expected init log file to exist, got %v", err)
	}
	beforeSize := beforeInfo.Size()

	if err := service.Sync(); err != nil {
		t.Fatalf("expected sync to succeed, got %v", err)
	}

	afterInfo, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("expected sync log file to exist, got %v", err)
	}
	afterSize := afterInfo.Size()
	if afterSize != beforeSize {
		t.Fatalf("expected sync without changes to keep log size unchanged (before=%d after=%d)", beforeSize, afterSize)
	}
}

func TestSyncAppendsOnlyChangedFilesToIndexLog(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(rootDir, ".git"), 0750); err != nil {
		t.Fatalf("expected git marker creation to succeed, got %v", err)
	}

	fileA := filepath.Join(rootDir, "a.txt")
	fileB := filepath.Join(rootDir, "b.txt")
	if err := os.WriteFile(fileA, []byte("alpha v1\n"), 0600); err != nil {
		t.Fatalf("expected file A creation to succeed, got %v", err)
	}
	if err := os.WriteFile(fileB, []byte("beta v1\n"), 0600); err != nil {
		t.Fatalf("expected file B creation to succeed, got %v", err)
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
		t.Fatalf("expected init to succeed, got %v", err)
	}

	logPath := filepath.Join(rootDir, ".idx", "logs", "tlog.idx")
	initialContent, err := os.ReadFile(logPath) //nolint:gosec
	if err != nil {
		t.Fatalf("expected init log file read to succeed, got %v", err)
	}

	if err := os.WriteFile(fileA, []byte("alpha v2\n"), 0600); err != nil {
		t.Fatalf("expected file A mutation before sync to succeed, got %v", err)
	}

	if err := service.Sync(); err != nil {
		t.Fatalf("expected sync to succeed, got %v", err)
	}

	updatedContent, err := os.ReadFile(logPath) //nolint:gosec
	if err != nil {
		t.Fatalf("expected updated log file read to succeed, got %v", err)
	}

	initialText := string(initialContent)
	updatedText := string(updatedContent)
	aPattern := string(filepath.Separator) + filepath.Base(fileA) + "\thash="
	bPattern := string(filepath.Separator) + filepath.Base(fileB) + "\thash="
	if strings.Count(initialText, aPattern) != 1 || strings.Count(initialText, bPattern) != 1 {
		t.Fatalf("expected initial log to contain one entry for each file, got %q", initialText)
	}

	if strings.Count(updatedText, aPattern) != 2 {
		t.Fatalf("expected changed file A to have two log entries after sync, got %q", updatedText)
	}

	if strings.Count(updatedText, bPattern) != 1 {
		t.Fatalf("expected unchanged file B to keep one log entry after sync, got %q", updatedText)
	}
}

func TestSyncAppendsOnlyNewFileToIndexLog(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(rootDir, ".git"), 0750); err != nil {
		t.Fatalf("expected git marker creation to succeed, got %v", err)
	}

	fileA := filepath.Join(rootDir, "a.txt")
	fileB := filepath.Join(rootDir, "b.txt")
	if err := os.WriteFile(fileA, []byte("alpha v1\n"), 0600); err != nil {
		t.Fatalf("expected file A creation to succeed, got %v", err)
	}
	if err := os.WriteFile(fileB, []byte("beta v1\n"), 0600); err != nil {
		t.Fatalf("expected file B creation to succeed, got %v", err)
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
		t.Fatalf("expected init to succeed, got %v", err)
	}

	logPath := filepath.Join(rootDir, ".idx", "logs", "tlog.idx")
	initialContent, err := os.ReadFile(logPath) //nolint:gosec
	if err != nil {
		t.Fatalf("expected init log file read to succeed, got %v", err)
	}

	fileC := filepath.Join(rootDir, "c.txt")
	if err := os.WriteFile(fileC, []byte("charlie v1\n"), 0600); err != nil {
		t.Fatalf("expected new file C creation before sync to succeed, got %v", err)
	}

	if err := service.Sync(); err != nil {
		t.Fatalf("expected sync to succeed, got %v", err)
	}

	updatedContent, err := os.ReadFile(logPath) //nolint:gosec
	if err != nil {
		t.Fatalf("expected updated log file read to succeed, got %v", err)
	}

	initialText := string(initialContent)
	updatedText := string(updatedContent)
	aPattern := string(filepath.Separator) + filepath.Base(fileA) + "\thash="
	bPattern := string(filepath.Separator) + filepath.Base(fileB) + "\thash="
	cPattern := string(filepath.Separator) + filepath.Base(fileC) + "\thash="

	if strings.Count(initialText, aPattern) != 1 || strings.Count(initialText, bPattern) != 1 {
		t.Fatalf("expected initial log to contain one entry for A and B, got %q", initialText)
	}

	if strings.Count(updatedText, aPattern) != 1 {
		t.Fatalf("expected unchanged file A to keep one log entry after sync, got %q", updatedText)
	}

	if strings.Count(updatedText, bPattern) != 1 {
		t.Fatalf("expected unchanged file B to keep one log entry after sync, got %q", updatedText)
	}

	if strings.Count(updatedText, cPattern) != 1 {
		t.Fatalf("expected new file C to be appended exactly once to log after sync, got %q", updatedText)
	}
}
