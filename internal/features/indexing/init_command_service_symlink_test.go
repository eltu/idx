package indexing_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"idx/internal/features/indexing"
	"idx/internal/features/indexing/storage"
	"idx/internal/shared/filesystem"
)

func TestInit_SkipsSymlinkPointingToDirectory(t *testing.T) {
	// NOTE: no t.Parallel() — uses os.Chdir which mutates process-wide state.

	// Arrange
	rootDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(rootDir, ".git"), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(rootDir, ".claude", "target"), 0o750))

	symlinkPath := filepath.Join(rootDir, ".claude", "skills")
	require.NoError(t, os.Symlink(filepath.Join(rootDir, ".claude", "target"), symlinkPath))

	rootFile := filepath.Join(rootDir, "README.txt")
	require.NoError(t, os.WriteFile(rootFile, []byte("hello"), 0o600))

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

	// Act
	err = service.Run()

	// Assert
	require.NoError(t, err, "expected init to succeed when directory symlink is present")
}
