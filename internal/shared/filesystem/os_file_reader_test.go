package filesystem

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOSFileReader_ReadFile_ReturnsContent(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	filePath := filepath.Join(dir, "a.txt")
	content := []byte("alpha\nbeta")
	tree := NewOSProjectTree()
	require.NoError(t, tree.WriteFile(filePath, content))
	reader := NewOSFileReader()

	// Act
	loaded, err := reader.ReadFile(filePath)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, string(content), loaded)
}

func TestOSFileReader_ReadFile_ReturnsErrorForMissingFile(t *testing.T) {
	t.Parallel()

	// Arrange
	reader := NewOSFileReader()

	// Act
	_, err := reader.ReadFile(filepath.Join(t.TempDir(), "missing.txt"))

	// Assert
	require.Error(t, err)
}
