package indexing

import (
	"path/filepath"
	"strings"

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

// BuildIndex constructs a BM25 inverted index from document metadata and content.
// Example: index, _ := service.BuildIndex([]domain.IndexDocument{{Name: "file.txt", Path: "./file.txt", Content: "hello world"}}).
func (service BM25IndexService) BuildIndex(documents []domain.IndexDocument) (*domain.InvertedIndex, error) {
	if len(documents) == 0 {
		return domain.NewInvertedIndex(), nil
	}

	index := domain.NewInvertedIndex()

	// First pass: tokenize all documents and collect term statistics
	tokensByDoc := make(map[string][]domain.TokenWithPosition)
	for _, document := range documents {
		tokens := domain.TokenizeText(document.Content)
		tokensByDoc[document.Name] = tokens
		index.AddDocument(document.Name, document.Path, len(tokens))
		index.AddPathTerms(document.Name, document.Path)
		index.AddFileNameTerms(document.Name, document.Name)
		index.AddExtensionTerms(document.Name, normalizedExtension(filepath.Ext(document.Name)))
	}

	// Second pass: build term index from content
	for docName, tokens := range tokensByDoc {
		frequencies, positions := domain.CountTokenFrequencies(tokens)

		for term, freq := range frequencies {
			index.AddTerm(term, docName, freq, positions[term])
		}
	}

	// Third pass: index filename tokens into BM25 Terms for retrieval.
	// Only adds terms not already present in content so content-based scores
	// are not distorted. Documents whose name contains a term will be found
	// even when that term never appears in their content.
	for _, document := range documents {
		fileNameTokens := domain.TokenizeFileName(document.Name)
		fileNameFreqs, fileNamePositions := domain.CountTokenFrequencies(fileNameTokens)
		for term, freq := range fileNameFreqs {
			if termStats := index.Terms[term]; termStats != nil {
				if _, alreadyIndexed := termStats.Docs[document.Name]; alreadyIndexed {
					continue
				}
			}
			index.AddTerm(term, document.Name, freq, fileNamePositions[term])
		}
	}

	// Fourth pass: calculate IDF scores
	index.CalculateAverageDocLen()
	index.CalculateIDF()

	return index, nil
}

func normalizedExtension(extension string) string {
	trimmed := strings.TrimSpace(extension)
	if trimmed == "" {
		return ""
	}

	return strings.TrimPrefix(strings.ToLower(trimmed), ".")
}
