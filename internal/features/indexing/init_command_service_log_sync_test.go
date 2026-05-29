package indexing_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"idx/internal/features/indexing"
	"idx/internal/features/indexing/storage"
	"idx/internal/shared/filesystem"
)

func TestInitAndSync_AppendIndexLog_WhenFilesChange(t *testing.T) {
	// NOTE: no t.Parallel() — uses os.Chdir which mutates process-wide state.

	// Arrange
	rootDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(rootDir, ".git"), 0750))

	filePath := filepath.Join(rootDir, "doc.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("needle v1\n"), 0600))

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
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(originalDir) })
	require.NoError(t, os.Chdir(rootDir))

	// Act: initial index
	require.NoError(t, service.Run())

	logPath := filepath.Join(rootDir, ".idx", "logs", "tlog.idx")
	beforeInfo, err := os.Stat(logPath)
	require.NoError(t, err)
	beforeSize := beforeInfo.Size()
	assert.Greater(t, beforeSize, int64(0))

	// Mutate file and sync
	require.NoError(t, os.WriteFile(filePath, []byte("needle v2\n"), 0600))
	require.NoError(t, service.Sync())

	// Assert: log grew
	afterInfo, err := os.Stat(logPath)
	require.NoError(t, err)
	assert.Greater(t, afterInfo.Size(), beforeSize)
}

func TestSync_DoesNotAppendIndexLog_WhenFilesUnchanged(t *testing.T) {
	// NOTE: no t.Parallel() — uses os.Chdir which mutates process-wide state.

	// Arrange
	rootDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(rootDir, ".git"), 0750))

	filePath := filepath.Join(rootDir, "doc.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("needle stable\n"), 0600))

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
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(originalDir) })
	require.NoError(t, os.Chdir(rootDir))

	require.NoError(t, service.Run())

	logPath := filepath.Join(rootDir, ".idx", "logs", "tlog.idx")
	beforeInfo, err := os.Stat(logPath)
	require.NoError(t, err)
	beforeSize := beforeInfo.Size()

	// Act: sync with no changes
	require.NoError(t, service.Sync())

	// Assert: log size unchanged
	afterInfo, err := os.Stat(logPath)
	require.NoError(t, err)
	assert.Equal(t, beforeSize, afterInfo.Size())
}

func TestSync_AppendsOnlyChangedFiles_ToIndexLog(t *testing.T) {
	// NOTE: no t.Parallel() — uses os.Chdir which mutates process-wide state.

	// Arrange
	rootDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(rootDir, ".git"), 0750))

	fileA := filepath.Join(rootDir, "a.txt")
	fileB := filepath.Join(rootDir, "b.txt")
	require.NoError(t, os.WriteFile(fileA, []byte("alpha v1\n"), 0600))
	require.NoError(t, os.WriteFile(fileB, []byte("beta v1\n"), 0600))

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
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(originalDir) })
	require.NoError(t, os.Chdir(rootDir))

	require.NoError(t, service.Run())

	logPath := filepath.Join(rootDir, ".idx", "logs", "tlog.idx")
	initialContent, err := os.ReadFile(logPath) //nolint:gosec
	require.NoError(t, err)

	// Act: only change file A
	require.NoError(t, os.WriteFile(fileA, []byte("alpha v2\n"), 0600))
	require.NoError(t, service.Sync())

	updatedContent, err := os.ReadFile(logPath) //nolint:gosec
	require.NoError(t, err)

	// Assert
	initialText := string(initialContent)
	updatedText := string(updatedContent)
	aPattern := string(filepath.Separator) + filepath.Base(fileA) + "\thash="
	bPattern := string(filepath.Separator) + filepath.Base(fileB) + "\thash="

	assert.Equal(t, 1, strings.Count(initialText, aPattern))
	assert.Equal(t, 1, strings.Count(initialText, bPattern))
	assert.Equal(t, 2, strings.Count(updatedText, aPattern), "expected changed file A to have two log entries after sync")
	assert.Equal(t, 1, strings.Count(updatedText, bPattern), "expected unchanged file B to keep one log entry after sync")
}

func TestSync_AppendsOnlyNewFile_ToIndexLog(t *testing.T) {
	// NOTE: no t.Parallel() — uses os.Chdir which mutates process-wide state.

	// Arrange
	rootDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(rootDir, ".git"), 0750))

	fileA := filepath.Join(rootDir, "a.txt")
	fileB := filepath.Join(rootDir, "b.txt")
	require.NoError(t, os.WriteFile(fileA, []byte("alpha v1\n"), 0600))
	require.NoError(t, os.WriteFile(fileB, []byte("beta v1\n"), 0600))

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
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(originalDir) })
	require.NoError(t, os.Chdir(rootDir))

	require.NoError(t, service.Run())

	logPath := filepath.Join(rootDir, ".idx", "logs", "tlog.idx")
	initialContent, err := os.ReadFile(logPath) //nolint:gosec
	require.NoError(t, err)

	// Act: add a new file C, then sync
	fileC := filepath.Join(rootDir, "c.txt")
	require.NoError(t, os.WriteFile(fileC, []byte("charlie v1\n"), 0600))
	require.NoError(t, service.Sync())

	updatedContent, err := os.ReadFile(logPath) //nolint:gosec
	require.NoError(t, err)

	// Assert
	initialText := string(initialContent)
	updatedText := string(updatedContent)
	aPattern := string(filepath.Separator) + filepath.Base(fileA) + "\thash="
	bPattern := string(filepath.Separator) + filepath.Base(fileB) + "\thash="
	cPattern := string(filepath.Separator) + filepath.Base(fileC) + "\thash="

	assert.Equal(t, 1, strings.Count(initialText, aPattern))
	assert.Equal(t, 1, strings.Count(initialText, bPattern))
	assert.Equal(t, 1, strings.Count(updatedText, aPattern), "expected unchanged file A to keep one log entry after sync")
	assert.Equal(t, 1, strings.Count(updatedText, bPattern), "expected unchanged file B to keep one log entry after sync")
	assert.Equal(t, 1, strings.Count(updatedText, cPattern), "expected new file C to be appended exactly once to log after sync")
}
