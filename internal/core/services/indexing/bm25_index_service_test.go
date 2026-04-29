package indexing_test

import (
	"path/filepath"
	"testing"

	"idx/internal/core/domain"
	"idx/internal/core/services/indexing"
)

func TestBM25IndexServiceBuildIndexBuildsDocumentsTermsAndPathMetadata(t *testing.T) {
	service := indexing.NewBM25IndexService()
	docs := []domain.IndexDocument{
		{Name: "a.txt", Path: filepath.Join("repo", "a.txt"), Content: "Go go idx"},
		{Name: "b.txt", Path: filepath.Join("repo", "sub", "b.txt"), Content: "idx search"},
	}

	index, err := service.BuildIndex(docs)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if index.DocumentCount != 2 {
		t.Fatalf("expected 2 documents, got %d", index.DocumentCount)
	}
	if index.Terms["go"] == nil || index.Terms["go"].Docs["a.txt"].TF != 2 {
		t.Fatal("expected go term frequency in a.txt")
	}
	if index.Terms["idx"] == nil || len(index.Terms["idx"].Docs) != 2 {
		t.Fatal("expected idx term in both docs")
	}
	if !index.PathTerms["repo"]["a.txt"] || !index.PathTerms["a.txt"]["a.txt"] || !index.PathTerms["sub"]["b.txt"] || !index.PathTerms["b.txt"]["b.txt"] {
		t.Fatal("expected path metadata segment tokens to be indexed")
	}
}

func TestBM25IndexServiceBuildIndexWithEmptyDocuments(t *testing.T) {
	service := indexing.NewBM25IndexService()
	index, err := service.BuildIndex([]domain.IndexDocument{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if index.DocumentCount != 0 || len(index.Terms) != 0 {
		t.Fatalf("expected empty index, got %+v", index)
	}
}
