package storage

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIndexFilePath_BuildsStandardPath(t *testing.T) {
	t.Parallel()

	// Arrange
	directoryPath := filepath.Join("repo", "pkg")
	expected := filepath.Join(directoryPath, ".idx", "index.idx")

	// Act & Assert
	assert.Equal(t, expected, indexFilePath(directoryPath))
}
