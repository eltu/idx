package repository

import (
	"path/filepath"
	"testing"

	"idx/internal/core/domain"
)

func TestJSONIndexRepositorySaveAndLoadIndex(t *testing.T) {
	tree := NewOSProjectTree()
	repo := NewJSONIndexRepository(tree)
	dir := t.TempDir()

	index := domain.NewInvertedIndex()
	index.AddDocument("readme.md", filepath.Join(dir, "readme.md"), 3)
	index.AddTerm("idx", "readme.md", 1, []int{7})
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
	if loaded.Documents["readme.md"] == nil {
		t.Fatal("expected document metadata in loaded index")
	}
}

func TestJSONIndexRepositoryLoadIndexReturnsErrorForMissingFile(t *testing.T) {
	tree := NewOSProjectTree()
	repo := NewJSONIndexRepository(tree)
	_, err := repo.LoadIndex(t.TempDir())
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
