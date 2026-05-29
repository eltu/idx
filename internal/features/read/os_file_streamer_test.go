package read_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"idx/internal/features/read"
)

func TestOSFileStreamer_OpenFile_ReadsContent(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.txt")
	require.NoError(t, os.WriteFile(path, []byte("hello\nworld\n"), 0600))
	streamer := read.NewOSFileStreamer()

	// Act
	rc, err := streamer.OpenFile(path)
	require.NoError(t, err)
	defer rc.Close()
	content, err := io.ReadAll(rc)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "hello\nworld\n", string(content))
}

func TestOSFileStreamer_OpenFile_ReturnsErrorForMissingFile(t *testing.T) {
	t.Parallel()

	// Arrange
	streamer := read.NewOSFileStreamer()

	// Act
	_, err := streamer.OpenFile(filepath.Join(t.TempDir(), "missing.txt"))

	// Assert
	require.Error(t, err)
}

func TestOSFileStreamer_IsDir_ReturnsTrueForDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	streamer := read.NewOSFileStreamer()

	// Act
	isDir, err := streamer.IsDir(t.TempDir())

	// Assert
	require.NoError(t, err)
	assert.True(t, isDir)
}

func TestOSFileStreamer_IsDir_ReturnsFalseForFile(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	require.NoError(t, os.WriteFile(path, []byte("x"), 0600))
	streamer := read.NewOSFileStreamer()

	// Act
	isDir, err := streamer.IsDir(path)

	// Assert
	require.NoError(t, err)
	assert.False(t, isDir)
}

func TestOSFileStreamer_IsDir_ReturnsErrorForMissingPath(t *testing.T) {
	t.Parallel()

	// Arrange
	streamer := read.NewOSFileStreamer()

	// Act
	_, err := streamer.IsDir(filepath.Join(t.TempDir(), "ghost"))

	// Assert
	require.Error(t, err)
}
