package indexstore

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"idx/internal/core/domain"
)

func TestBinaryIndexRepositorySaveAndLoadIndex(t *testing.T) {
	repo := NewBinaryIndexRepository()
	dir := t.TempDir()

	index := domain.NewInvertedIndex()
	index.AddDocument("a.txt", filepath.Join(dir, "a.txt"), 2)
	index.AddTerm("go", "a.txt", 2, []int{0, 3})
	index.CalculateAverageDocLen()
	index.CalculateIDF()

	if err := repo.SaveIndex(dir, index); err != nil {
		t.Fatalf("expected save to succeed, got %v", err)
	}

	loaded, err := repo.LoadIndex(dir)
	if err != nil {
		t.Fatalf("expected load to succeed, got %v", err)
	}

	if loaded.DocumentCount != 1 {
		t.Fatalf("expected document count 1, got %d", loaded.DocumentCount)
	}
	if loaded.Terms["go"] == nil || loaded.Terms["go"].Docs["a.txt"] == nil {
		t.Fatal("expected term stats to be present after load")
	}
}

func TestBinaryIndexRepositoryLoadIndexReturnsErrorForMissingFile(t *testing.T) {
	repo := NewBinaryIndexRepository()
	_, err := repo.LoadIndex(t.TempDir())
	if err == nil {
		t.Fatal("expected error for missing index file, got nil")
	}
}

func TestBinaryIndexRepositoryLoadIndexReturnsErrorForInvalidBinaryPayload(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, ".idx", "index.idx")
	if err := os.MkdirAll(filepath.Dir(indexPath), 0750); err != nil {
		t.Fatalf("expected index directory creation to succeed, got %v", err)
	}
	if err := os.WriteFile(indexPath, []byte("not-gob"), 0600); err != nil {
		t.Fatalf("expected invalid payload write to succeed, got %v", err)
	}

	repo := NewBinaryIndexRepository()
	_, err := repo.LoadIndex(dir)
	if err == nil {
		t.Fatal("expected parse error for invalid binary payload")
	}
}

func TestBinaryIndexRepositorySaveIndexReturnsErrorForInvalidDirectory(t *testing.T) {
	repo := NewBinaryIndexRepository()
	index := domain.NewInvertedIndex()

	err := repo.SaveIndex("\x00invalid", index)
	if err == nil {
		t.Fatal("expected save error for invalid directory path")
	}
}

func TestBinaryIndexRepositorySaveIndexReturnsErrorWhenEncodingNilIndex(t *testing.T) {
	repo := NewBinaryIndexRepository()
	dir := t.TempDir()

	err := repo.SaveIndex(dir, nil)
	if err == nil {
		t.Fatal("expected serialize error for nil index")
	}
}

func TestBinaryIndexRepositorySaveIndexReturnsErrorWhenRepositoryIsNil(t *testing.T) {
	var repo *BinaryIndexRepository
	index := domain.NewInvertedIndex()

	err := repo.SaveIndex(t.TempDir(), index)
	if err == nil {
		t.Fatal("expected save error for nil repository receiver")
	}
}

func TestBinaryIndexRepositoryLoadIndexReturnsErrorWhenRepositoryIsNil(t *testing.T) {
	var repo *BinaryIndexRepository

	_, err := repo.LoadIndex(t.TempDir())
	if err == nil {
		t.Fatal("expected load error for nil repository receiver")
	}
}

func TestBinaryIndexRepositorySaveIndexReturnsErrorWhenTargetIsDirectory(t *testing.T) {
	repo := NewBinaryIndexRepository()
	dir := t.TempDir()

	indexPath := filepath.Join(dir, ".idx", "index.idx")
	if err := os.MkdirAll(indexPath, 0750); err != nil {
		t.Fatalf("expected directory creation at target path, got %v", err)
	}

	index := domain.NewInvertedIndex()
	err := repo.SaveIndex(dir, index)
	if err == nil {
		t.Fatal("expected rename error when target index path is directory")
	}
}

func TestBinaryIndexRepositorySaveIndexReturnsErrorWhenTempFileCannotBeCreated(t *testing.T) {
	repo := NewBinaryIndexRepository()
	root := t.TempDir()
	indexDir := filepath.Join(root, ".idx")
	if err := os.MkdirAll(indexDir, 0750); err != nil {
		t.Fatalf("expected index dir creation, got %v", err)
	}

	if err := os.Chmod(indexDir, 0500); err != nil {
		t.Fatalf("expected chmod to read/execute only, got %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(indexDir, 0750) })

	index := domain.NewInvertedIndex()
	err := repo.SaveIndex(root, index)
	if err == nil {
		t.Fatal("expected temp file creation error when directory is not writable")
	}
}

func TestBinaryIndexRepositoryConcurrentSaveAndLoad(t *testing.T) {
	repo := NewBinaryIndexRepository()
	dir := t.TempDir()

	seed := domain.NewInvertedIndex()
	seed.AddDocument("seed.txt", filepath.Join(dir, "seed.txt"), 1)
	seed.AddTerm("seed", "seed.txt", 1, []int{0})
	seed.CalculateAverageDocLen()
	seed.CalculateIDF()
	if err := repo.SaveIndex(dir, seed); err != nil {
		t.Fatalf("expected seed save to succeed, got %v", err)
	}

	const iterations = 120
	errorsCh := make(chan error, iterations*2)

	var workers sync.WaitGroup
	workers.Add(2)

	go func() {
		defer workers.Done()
		for index := 0; index < iterations; index++ {
			current := domain.NewInvertedIndex()
			name := fmt.Sprintf("doc-%03d.txt", index)
			current.AddDocument(name, filepath.Join(dir, name), 2)
			current.AddTerm("needle", name, 1, []int{index})
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
		for index := 0; index < iterations; index++ {
			loaded, err := repo.LoadIndex(dir)
			if err != nil {
				errorsCh <- err
				return
			}

			if loaded.DocumentCount < 1 {
				errorsCh <- fmt.Errorf("expected at least one document in loaded index, got %d", loaded.DocumentCount)
				return
			}
		}
	}()

	workers.Wait()
	close(errorsCh)
	for err := range errorsCh {
		t.Fatalf("expected concurrent save/load without corruption, got %v", err)
	}
}
