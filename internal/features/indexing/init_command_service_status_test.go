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

func TestStatus_ReportsIndicesUpToDate_WhenLatestLogsMatchFileTimestamp(t *testing.T) {
	// NOTE: no t.Parallel() — uses os.Chdir which mutates process-wide state.

	// Arrange
	rootDir := t.TempDir()
	ensureGitProject(t, rootDir)

	rootFile := filepath.Join(rootDir, "root.txt")
	nestedDir := filepath.Join(rootDir, "internal")
	nestedFile := filepath.Join(nestedDir, "service.txt")
	writeFile(t, rootFile, "root v1")
	writeFile(t, nestedFile, "service v1")

	output := &capturingTextOutput{}
	service := newStatusService(output)
	changeTo(t, rootDir)

	require.NoError(t, service.Run())

	// Act
	err := service.Status()

	// Assert
	require.NoError(t, err)
	require.NotEmpty(t, output.lines)
	outputText := strings.Join(output.lines, "")
	assert.Contains(t, outputText, "up to date")
}

func TestStatus_WithProfile_ReportsDetailedTableAndSummary(t *testing.T) {
	// NOTE: no t.Parallel() — uses os.Chdir which mutates process-wide state.

	// Arrange
	rootDir := t.TempDir()
	ensureGitProject(t, rootDir)

	rootFile := filepath.Join(rootDir, "root.txt")
	nestedDir := filepath.Join(rootDir, "internal")
	nestedFile := filepath.Join(nestedDir, "service.txt")
	writeFile(t, rootFile, "root v1")
	writeFile(t, nestedFile, "service v1")

	output := &capturingTextOutput{}
	service := newStatusService(output)
	changeTo(t, rootDir)

	require.NoError(t, service.Run())

	// Act
	err := service.StatusWithProfile(true)

	// Assert
	require.NoError(t, err)
	outputText := strings.Join(output.lines, "\n")
	assert.Contains(t, outputText, "📊 Summary")
	assert.Contains(t, outputText, "files checked")
	assert.Contains(t, outputText, "root.txt")
	assert.Contains(t, outputText, "✓")
}

func TestStatus_FailsWhenFileChangedAfterLastIndex(t *testing.T) {
	// NOTE: no t.Parallel() — uses os.Chdir which mutates process-wide state.

	// Arrange
	rootDir := t.TempDir()
	ensureGitProject(t, rootDir)

	rootFile := filepath.Join(rootDir, "root.txt")
	writeFile(t, rootFile, "root v1")

	output := &capturingTextOutput{}
	service := newStatusService(output)
	changeTo(t, rootDir)

	require.NoError(t, service.Run())
	linesBeforeStatus := len(output.lines)

	writeFile(t, rootFile, "root v2 — changed after indexing")

	// Act
	err := service.Status()

	// Assert
	require.Error(t, err)
	assert.ErrorContains(t, err, "stale index")
	assert.Greater(t, len(output.lines), linesBeforeStatus)
	outputText := strings.Join(output.lines[linesBeforeStatus:], "\n")
	assert.Contains(t, outputText, "idx sync")
}

func TestStatus_FailsWhenNewDirectoryIsNotIndexed(t *testing.T) {
	// NOTE: no t.Parallel() — uses os.Chdir which mutates process-wide state.

	// Arrange
	rootDir := t.TempDir()
	ensureGitProject(t, rootDir)

	rootFile := filepath.Join(rootDir, "root.txt")
	writeFile(t, rootFile, "root v1")

	service := newStatusService(&capturingTextOutput{})
	changeTo(t, rootDir)

	require.NoError(t, service.Run())

	// Add a new subdirectory with a file after indexing — it should be detected as missing.
	newDir := filepath.Join(rootDir, "newpkg")
	writeFile(t, filepath.Join(newDir, "handler.txt"), "new content")

	// Act
	err := service.Status()

	// Assert
	require.Error(t, err)
	assert.ErrorContains(t, err, "unindexed")
}

func newStatusService(output *capturingTextOutput) indexing.InitCommandService {
	return indexing.NewInitCommandService(indexing.InitCommandServiceDeps{
		ProjectTree:    filesystem.NewOSProjectTree(),
		MatcherFactory: fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}},
		Output:         output,
		FileReader:     filesystem.NewOSFileReader(),
		Indexer:        indexing.NewBM25IndexService(),
		IndexRepo:      storage.NewBinaryIndexRepository(),
		ChecksumRepo:   storage.NewDirectoryChecksumRepository(),
		DaemonRepo:     nil,
	})
}

func ensureGitProject(t *testing.T, rootDir string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(rootDir, ".git"), 0o750))
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func changeTo(t *testing.T, directoryPath string) {
	t.Helper()
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(originalDir) })
	require.NoError(t, os.Chdir(directoryPath))
}
