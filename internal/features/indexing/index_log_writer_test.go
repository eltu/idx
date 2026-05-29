package indexing

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- appendIndexedFilesLog ----

func TestAppendIndexedFilesLog_EmptyEntries_IsNoop(t *testing.T) {
	t.Parallel()

	// Act
	err := appendIndexedFilesLog(t.TempDir(), nil)

	// Assert
	require.NoError(t, err)
}

func TestAppendIndexedFilesLog_NonExistentDir_IsNoop(t *testing.T) {
	t.Parallel()

	// Act
	err := appendIndexedFilesLog("/nonexistent/xyz/abc", []indexedFileLogEntry{{Path: "x"}})

	// Assert
	require.NoError(t, err)
}

func TestAppendIndexedFilesLog_ValidEntries_CreatesLogFile(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	entries := []indexedFileLogEntry{
		{Path: "main.go", Checksum: "abc123", IndexedAt: time.Now()},
	}

	// Act
	err := appendIndexedFilesLog(dir, entries)

	// Assert
	require.NoError(t, err)
	logPath := filepath.Join(dir, ".idx", "logs", "tlog.idx")
	content, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "main.go")
}

func TestAppendIndexedFilesLog_MultipleCalls_AppendsEntries(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	entries1 := []indexedFileLogEntry{{Path: "a.go", Checksum: "h1", IndexedAt: time.Now()}}
	entries2 := []indexedFileLogEntry{{Path: "b.go", Checksum: "h2", IndexedAt: time.Now()}}

	// Act
	require.NoError(t, appendIndexedFilesLog(dir, entries1))
	require.NoError(t, appendIndexedFilesLog(dir, entries2))

	// Assert
	logPath := filepath.Join(dir, ".idx", "logs", "tlog.idx")
	content, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "a.go")
	assert.Contains(t, string(content), "b.go")
}

// ---- renderLogEntries ----

func TestRenderLogEntries_Format_IsCorrect(t *testing.T) {
	t.Parallel()

	// Arrange
	ts := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	entries := []indexedFileLogEntry{{Path: "internal/svc.go", Checksum: "deadbeef", IndexedAt: ts}}

	// Act
	payload := renderLogEntries(entries)

	// Assert
	line := string(payload)
	assert.Contains(t, line, "path=internal/svc.go")
	assert.Contains(t, line, "hash=deadbeef")
	assert.Contains(t, line, "2026-01-15")
}

func TestRenderLogEntries_MultipleEntries_HaveNewlines(t *testing.T) {
	t.Parallel()

	// Arrange
	entries := []indexedFileLogEntry{
		{Path: "a.go", Checksum: "h1", IndexedAt: time.Now()},
		{Path: "b.go", Checksum: "h2", IndexedAt: time.Now()},
	}

	// Act
	payload := renderLogEntries(entries)

	// Assert
	lines := strings.Split(strings.TrimRight(string(payload), "\n"), "\n")
	assert.Len(t, lines, 2)
}

// ---- directoryExists ----

func TestDirectoryExists_ExistingDir_ReturnsTrue(t *testing.T) {
	t.Parallel()
	assert.True(t, directoryExists(t.TempDir()))
}

func TestDirectoryExists_MissingPath_ReturnsFalse(t *testing.T) {
	t.Parallel()
	assert.False(t, directoryExists("/nonexistent/xyz"))
}

func TestDirectoryExists_FilePath_ReturnsFalse(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	f := filepath.Join(dir, "file.txt")
	require.NoError(t, os.WriteFile(f, []byte("x"), 0600))

	// Assert
	assert.False(t, directoryExists(f))
}

// ---- shouldRotateActiveLog ----

func TestShouldRotateActiveLog_FileMissing_ReturnsFalse(t *testing.T) {
	t.Parallel()
	assert.False(t, shouldRotateActiveLog("/nonexistent/tlog.idx", 100))
}

func TestShouldRotateActiveLog_SmallFile_ReturnsFalse(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	path := filepath.Join(dir, "tlog.idx")
	require.NoError(t, os.WriteFile(path, []byte("small"), 0600))

	// Assert
	assert.False(t, shouldRotateActiveLog(path, 10))
}

func TestShouldRotateActiveLog_AtMaxSize_ReturnsTrue(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	path := filepath.Join(dir, "tlog.idx")
	content := make([]byte, indexLogMaxSizeBytes)
	require.NoError(t, os.WriteFile(path, content, 0600))

	// Assert
	assert.True(t, shouldRotateActiveLog(path, 1))
}

func TestShouldRotateActiveLog_WouldExceedMax_ReturnsTrue(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	path := filepath.Join(dir, "tlog.idx")
	content := make([]byte, indexLogMaxSizeBytes-10)
	require.NoError(t, os.WriteFile(path, content, 0600))

	// Assert
	assert.True(t, shouldRotateActiveLog(path, 100))
}

// ---- rotateActiveLogFile ----

func TestRotateActiveLogFile_ExistingFile_RenamesFile(t *testing.T) {
	t.Parallel()

	// Arrange
	logsDir := t.TempDir()
	activePath := filepath.Join(logsDir, "tlog.idx")
	require.NoError(t, os.WriteFile(activePath, []byte("log data"), 0600))

	// Act
	err := rotateActiveLogFile(logsDir, activePath)

	// Assert
	require.NoError(t, err)
	_, statErr := os.Stat(activePath)
	assert.Error(t, statErr, "expected active log to be renamed away")
	files, _ := filepath.Glob(filepath.Join(logsDir, "tlog_*.log"))
	assert.NotEmpty(t, files)
}

func TestRotateActiveLogFile_NonExistentFile_ReturnsError(t *testing.T) {
	t.Parallel()

	// Arrange
	logsDir := t.TempDir()
	activePath := filepath.Join(logsDir, "nonexistent.idx")

	// Act
	err := rotateActiveLogFile(logsDir, activePath)

	// Assert
	require.Error(t, err)
}

// ---- cleanupRotatedLogs ----

func TestCleanupRotatedLogs_OverLimit_RemovesOldest(t *testing.T) {
	t.Parallel()

	// Arrange
	logsDir := t.TempDir()
	for i := range indexLogMaxRotatedFiles + 3 {
		name := fmt.Sprintf("tlog_2026010%d120000.log", i)
		require.NoError(t, os.WriteFile(filepath.Join(logsDir, name), []byte("data"), 0600))
	}

	// Act
	err := cleanupRotatedLogs(logsDir)

	// Assert
	require.NoError(t, err)
	remaining, _ := filepath.Glob(filepath.Join(logsDir, "tlog_*.log"))
	assert.Len(t, remaining, indexLogMaxRotatedFiles)
}

func TestCleanupRotatedLogs_UnderLimit_DoesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	logsDir := t.TempDir()
	for i := range 3 {
		name := fmt.Sprintf("tlog_202601%02d120000.log", i+1)
		require.NoError(t, os.WriteFile(filepath.Join(logsDir, name), []byte("data"), 0600))
	}

	// Act
	err := cleanupRotatedLogs(logsDir)

	// Assert
	require.NoError(t, err)
	remaining, _ := filepath.Glob(filepath.Join(logsDir, "tlog_*.log"))
	assert.Len(t, remaining, 3)
}

// ---- appendIndexedFilesLog MkdirAll error ----

func TestAppendIndexedFilesLog_MkdirAllError_ReturnsError(t *testing.T) {
	t.Parallel()

	// Arrange: Create a file at the .idx path so MkdirAll(".idx/logs") fails.
	dir := t.TempDir()
	idxPath := filepath.Join(dir, ".idx")
	require.NoError(t, os.WriteFile(idxPath, []byte("not a directory"), 0600))
	entries := []indexedFileLogEntry{{Path: "main.go", Checksum: "abc", IndexedAt: time.Now()}}

	// Act
	err := appendIndexedFilesLog(dir, entries)

	// Assert
	require.Error(t, err)
}

// ---- appendIndexedFilesLog with rotation ----

func TestAppendIndexedFilesLog_AtMaxSize_RotatesLog(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	logsDir := filepath.Join(dir, ".idx", "logs")
	require.NoError(t, os.MkdirAll(logsDir, 0750))
	activePath := filepath.Join(logsDir, "tlog.idx")
	oversized := make([]byte, indexLogMaxSizeBytes)
	require.NoError(t, os.WriteFile(activePath, oversized, 0600))
	entries := []indexedFileLogEntry{{Path: "main.go", Checksum: "abc", IndexedAt: time.Now()}}

	// Act
	err := appendIndexedFilesLog(dir, entries)

	// Assert
	require.NoError(t, err)
	rotated, _ := filepath.Glob(filepath.Join(logsDir, "tlog_*.log"))
	assert.NotEmpty(t, rotated)
}
