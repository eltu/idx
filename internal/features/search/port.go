package search

import "idx/internal/features/indexing"

// IndexLoader loads a pre-built inverted index from a directory path.
// Example: repo.LoadIndex("/path/to/project/subdir").
type IndexLoader interface {
	LoadIndex(directoryPath string) (*indexing.InvertedIndex, error)
}
