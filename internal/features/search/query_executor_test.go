package search

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testProjectRoot = "/project"

func TestFilterByChangedFiles_KeepsOnlyChangedFiles(t *testing.T) {
	t.Parallel()

	// Arrange
	root := testProjectRoot
	results := []searchResult{
		{directoryPath: "/project", fileName: "a.go"},
		{directoryPath: "/project", fileName: "b.go"},
		{directoryPath: "/project/internal", fileName: "c.go"},
	}
	changed := map[string]bool{
		"a.go":          true,
		"internal/c.go": true,
	}

	// Act
	filtered := filterByChangedFiles(results, root, changed)

	// Assert
	require.Len(t, filtered, 2)
	names := []string{filtered[0].fileName, filtered[1].fileName}
	assert.Contains(t, names, "a.go")
	assert.Contains(t, names, "c.go")
}

func TestFilterByChangedFiles_NoMatches_ReturnsEmptySlice(t *testing.T) {
	t.Parallel()

	// Arrange
	root := testProjectRoot
	results := []searchResult{
		{directoryPath: "/project", fileName: "a.go"},
	}
	changed := map[string]bool{"other.go": true}

	// Act
	filtered := filterByChangedFiles(results, root, changed)

	// Assert
	assert.Empty(t, filtered)
}

func TestFilterByChangedFiles_EmptyResults_ReturnsEmptySlice(t *testing.T) {
	t.Parallel()

	// Arrange
	changed := map[string]bool{"a.go": true}

	// Act
	filtered := filterByChangedFiles([]searchResult{}, testProjectRoot, changed)

	// Assert
	assert.Empty(t, filtered)
}
