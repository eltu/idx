package services

import (
	"idx/internal/core/domain"
)

// BM25IndexService implements the BM25Indexer port.
// It builds an inverted index with BM25 scoring from text documents.
type BM25IndexService struct {
}

// NewBM25IndexService creates a new BM25 indexing service.
func NewBM25IndexService() BM25IndexService {
	return BM25IndexService{}
}

// BuildIndex constructs a BM25 inverted index from a map of documents.
// Example: index, _ := service.BuildIndex(map[string]string{"file.txt": "hello world"}).
func (service BM25IndexService) BuildIndex(documents map[string]string) (*domain.InvertedIndex, error) {
	if len(documents) == 0 {
		return domain.NewInvertedIndex(), nil
	}

	index := domain.NewInvertedIndex()

	// First pass: tokenize all documents and collect term statistics
	tokensByDoc := make(map[string][]domain.TokenWithPosition)
	for docName, content := range documents {
		tokens := domain.TokenizeText(content)
		tokensByDoc[docName] = tokens
		index.AddDocument(docName, len(tokens))
	}

	// Second pass: build term index
	for docName, tokens := range tokensByDoc {
		frequencies, positions := domain.CountTokenFrequencies(tokens)

		for term, freq := range frequencies {
			index.AddTerm(term, docName, freq, positions[term])
		}
	}

	// Third pass: calculate IDF scores
	index.CalculateAverageDocLen()
	index.CalculateIDF()

	return index, nil
}
