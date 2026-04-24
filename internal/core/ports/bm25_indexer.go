package ports

import "idx/internal/core/domain"

// FileReader reads text content from files in the filesystem.
type FileReader interface {
	ReadFile(path string) (string, error)
}

// BM25Indexer builds an inverted index with BM25 scoring from file documents.
type BM25Indexer interface {
	BuildIndex(documents []domain.IndexDocument) (*domain.InvertedIndex, error)
}
