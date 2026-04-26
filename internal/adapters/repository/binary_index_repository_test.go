package repository

import (
	"path/filepath"
	"testing"

	"idx/internal/core/domain"
)

func TestBinaryIndexRepositorySaveAndLoadIndex(t *testing.T) {
	tree := NewOSProjectTree()
	repo := NewBinaryIndexRepository(tree)
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
	tree := NewOSProjectTree()
	repo := NewBinaryIndexRepository(tree)
	_, err := repo.LoadIndex(t.TempDir())
	if err == nil {
		t.Fatal("expected error for missing index file, got nil")
	}
}
