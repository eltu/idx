package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"idx/internal/features/indexing"
)

func TestBinaryIndexRepository_SaveAndLoad_RoundTripsIndex(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := NewBinaryIndexRepository()
	dir := t.TempDir()
	index := indexing.NewInvertedIndex()
	index.AddDocument("a.txt", filepath.Join(dir, "a.txt"), 2)
	index.AddTerm("go", "a.txt", 2, []int{0, 3})
	index.CalculateAverageDocLen()
	index.CalculateIDF()

	// Act
	require.NoError(t, repo.SaveIndex(dir, index))
	loaded, err := repo.LoadIndex(dir)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 1, loaded.DocumentCount)
	require.NotNil(t, loaded.Terms["go"])
	assert.NotNil(t, loaded.Terms["go"].Docs["a.txt"])
}

func TestBinaryIndexRepository_LoadIndex_ReturnsErrorForMissingFile(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := NewBinaryIndexRepository()

	// Act
	_, err := repo.LoadIndex(t.TempDir())

	// Assert
	require.Error(t, err)
}

func TestBinaryIndexRepository_LoadIndex_ReturnsErrorForInvalidBinaryPayload(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	indexPath := filepath.Join(dir, ".idx", "index.idx")
	require.NoError(t, os.MkdirAll(filepath.Dir(indexPath), 0750))
	require.NoError(t, os.WriteFile(indexPath, []byte("not-gob"), 0600))
	repo := NewBinaryIndexRepository()

	// Act
	_, err := repo.LoadIndex(dir)

	// Assert
	require.Error(t, err)
}

func TestBinaryIndexRepository_SaveIndex_ReturnsErrorForInvalidDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := NewBinaryIndexRepository()

	// Act
	err := repo.SaveIndex("\x00invalid", indexing.NewInvertedIndex())

	// Assert
	require.Error(t, err)
}

func TestBinaryIndexRepository_SaveIndex_ReturnsErrorWhenEncodingNilIndex(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := NewBinaryIndexRepository()

	// Act
	err := repo.SaveIndex(t.TempDir(), nil)

	// Assert
	require.Error(t, err)
}

func TestBinaryIndexRepository_SaveIndex_ReturnsErrorWhenRepositoryIsNil(t *testing.T) {
	t.Parallel()

	// Arrange
	var repo *BinaryIndexRepository

	// Act
	err := repo.SaveIndex(t.TempDir(), indexing.NewInvertedIndex())

	// Assert
	require.Error(t, err)
}

func TestBinaryIndexRepository_LoadIndex_ReturnsErrorWhenRepositoryIsNil(t *testing.T) {
	t.Parallel()

	// Arrange
	var repo *BinaryIndexRepository

	// Act
	_, err := repo.LoadIndex(t.TempDir())

	// Assert
	require.Error(t, err)
}

func TestBinaryIndexRepository_SaveIndex_ReturnsErrorWhenTempFileCannotBeCreated(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := NewBinaryIndexRepository()
	root := t.TempDir()
	indexDir := filepath.Join(root, ".idx")
	require.NoError(t, os.MkdirAll(indexDir, 0750))
	require.NoError(t, os.Chmod(indexDir, 0500))
	t.Cleanup(func() { _ = os.Chmod(indexDir, 0750) })

	// Act
	err := repo.SaveIndex(root, indexing.NewInvertedIndex())

	// Assert
	require.Error(t, err)
}

func TestBinaryIndexRepository_ConcurrentSaveAndLoad_NoCorrption(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := NewBinaryIndexRepository()
	dir := t.TempDir()

	seed := indexing.NewInvertedIndex()
	seed.AddDocument("seed.txt", filepath.Join(dir, "seed.txt"), 1)
	seed.AddTerm("seed", "seed.txt", 1, []int{0})
	seed.CalculateAverageDocLen()
	seed.CalculateIDF()
	require.NoError(t, repo.SaveIndex(dir, seed))

	const iterations = 120
	errorsCh := make(chan error, iterations*2)
	var workers sync.WaitGroup
	workers.Add(2)

	go func() {
		defer workers.Done()
		for i := 0; i < iterations; i++ {
			current := indexing.NewInvertedIndex()
			name := fmt.Sprintf("doc-%03d.txt", i)
			current.AddDocument(name, filepath.Join(dir, name), 2)
			current.AddTerm("needle", name, 1, []int{i})
			current.CalculateAverageDocLen()
			current.CalculateIDF()
			if err := repo.SaveIndex(dir, current); err != nil {
				errorsCh <- err
				return
			}
		}
	}()

	go func() {
		defer workers.Done()
		for i := 0; i < iterations; i++ {
			loaded, err := repo.LoadIndex(dir)
			if err != nil {
				errorsCh <- err
				return
			}
			if loaded.DocumentCount < 1 {
				errorsCh <- fmt.Errorf("expected at least 1 document, got %d", loaded.DocumentCount)
				return
			}
		}
	}()

	workers.Wait()
	close(errorsCh)
	for err := range errorsCh {
		require.NoError(t, err, "concurrent save/load must not corrupt data")
	}
}
