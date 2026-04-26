package repository

import (
	"errors"
	"os"
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

func TestJSONIndexRepositoryLoadIndexReturnsErrorForInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, ".idx", "index.idx")
	if err := os.MkdirAll(filepath.Dir(indexPath), 0750); err != nil {
		t.Fatalf("expected index directory creation to succeed, got %v", err)
	}
	if err := os.WriteFile(indexPath, []byte("{invalid-json"), 0600); err != nil {
		t.Fatalf("expected invalid JSON write to succeed, got %v", err)
	}

	repo := NewJSONIndexRepository(NewOSProjectTree())
	_, err := repo.LoadIndex(dir)
	if err == nil {
		t.Fatal("expected parse error for invalid JSON payload")
	}
}

type failingWriteTree struct{}

func (failingWriteTree) CurrentDir() (string, error)                     { return "", nil }
func (failingWriteTree) FindGitRoot(string) (string, error)              { return "", nil }
func (failingWriteTree) ReadDir(string) ([]domain.DirectoryEntry, error) { return nil, nil }
func (failingWriteTree) Exists(string) (bool, error)                     { return false, nil }
func (failingWriteTree) RemoveAll(string) error                          { return nil }
func (failingWriteTree) WriteFile(string, []byte) error                  { return errors.New("write failed") }

func TestJSONIndexRepositorySaveIndexReturnsErrorWhenWriteFails(t *testing.T) {
	repo := NewJSONIndexRepository(failingWriteTree{})
	index := domain.NewInvertedIndex()
	if err := repo.SaveIndex(t.TempDir(), index); err == nil {
		t.Fatal("expected save error when projectTree write fails")
	}
}
