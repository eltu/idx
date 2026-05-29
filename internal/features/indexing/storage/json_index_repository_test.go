package storage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"idx/internal/features/indexing"
	"idx/internal/shared/filesystem"
)

func TestJSONIndexRepository_SaveAndLoad_RoundTripsIndex(t *testing.T) {
	t.Parallel()

	// Arrange
	tree := filesystem.NewOSProjectTree()
	repo := NewJSONIndexRepository(tree)
	dir := t.TempDir()
	index := indexing.NewInvertedIndex()
	index.AddDocument("readme.md", filepath.Join(dir, "readme.md"), 3)
	index.AddTerm("idx", "readme.md", 1, []int{7})
	index.CalculateAverageDocLen()
	index.CalculateIDF()

	// Act
	require.NoError(t, repo.SaveIndex(dir, index))
	loaded, err := repo.LoadIndex(dir)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 1, loaded.DocumentCount)
	assert.NotNil(t, loaded.Documents["readme.md"])
}

func TestJSONIndexRepository_LoadIndex_ReturnsErrorForMissingFile(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := NewJSONIndexRepository(filesystem.NewOSProjectTree())

	// Act
	_, err := repo.LoadIndex(t.TempDir())

	// Assert
	require.Error(t, err)
}

func TestJSONIndexRepository_LoadIndex_ReturnsErrorForInvalidJSON(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	indexPath := filepath.Join(dir, ".idx", "index.idx")
	require.NoError(t, os.MkdirAll(filepath.Dir(indexPath), 0750))
	require.NoError(t, os.WriteFile(indexPath, []byte("{invalid-json"), 0600))
	repo := NewJSONIndexRepository(filesystem.NewOSProjectTree())

	// Act
	_, err := repo.LoadIndex(dir)

	// Assert
	require.Error(t, err)
}

func TestJSONIndexRepository_SaveIndex_ReturnsErrorWhenWriteFails(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := NewJSONIndexRepository(failingWriteTree{})

	// Act
	err := repo.SaveIndex(t.TempDir(), indexing.NewInvertedIndex())

	// Assert
	require.Error(t, err)
}

func TestJSONIndexRepository_SaveIndex_ReturnsErrorForNilIndex(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := NewJSONIndexRepository(filesystem.NewOSProjectTree())

	// Act
	err := repo.SaveIndex(t.TempDir(), nil)

	// Assert
	require.Error(t, err)
}

func TestJSONIndexRepository_SaveIndex_ReturnsErrorWhenProjectTreeIsNil(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := NewJSONIndexRepository(nil)

	// Act
	err := repo.SaveIndex(t.TempDir(), indexing.NewInvertedIndex())

	// Assert
	require.Error(t, err)
}

func TestJSONIndexRepository_SaveIndex_ReturnsErrorWhenRepositoryIsNil(t *testing.T) {
	t.Parallel()

	// Arrange
	var repo *JSONIndexRepository

	// Act
	err := repo.SaveIndex(t.TempDir(), indexing.NewInvertedIndex())

	// Assert
	require.Error(t, err)
}

func TestJSONIndexRepository_LoadIndex_ReturnsErrorWhenRepositoryIsNil(t *testing.T) {
	t.Parallel()

	// Arrange
	var repo *JSONIndexRepository

	// Act
	_, err := repo.LoadIndex(t.TempDir())

	// Assert
	require.Error(t, err)
}

type failingWriteTree struct{}

func (failingWriteTree) CurrentDir() (string, error)                         { return "", nil }
func (failingWriteTree) FindGitRoot(string) (string, error)                  { return "", nil }
func (failingWriteTree) ReadDir(string) ([]filesystem.DirectoryEntry, error) { return nil, nil }
func (failingWriteTree) Exists(string) (bool, error)                         { return false, nil }
func (failingWriteTree) RemoveAll(string) error                              { return nil }
func (failingWriteTree) WriteFile(string, []byte) error                      { return errors.New("write failed") }
