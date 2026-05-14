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
	index, tokensByDoc := prepareDocumentIndex(documents)
	addTermsFromContent(index, tokensByDoc)
	addFileNameTermsToIndex(index, documents)
	index.CalculateAverageDocLen()
	index.CalculateIDF()
	return index, nil
}

// prepareDocumentIndex runs the first pass: tokenizes all documents and records
// metadata (path, extension) in the index.
func prepareDocumentIndex(documents []domain.IndexDocument) (*domain.InvertedIndex, map[string][]domain.TokenWithPosition) {
	index := domain.NewInvertedIndex()
	tokensByDoc := make(map[string][]domain.TokenWithPosition)
	for _, document := range documents {
		tokens := domain.TokenizeText(document.Content)
		tokensByDoc[document.Name] = tokens
		index.AddDocument(document.Name, document.Path, len(tokens))
		index.AddPathTerms(document.Name, document.Path)
		index.AddFileNameTerms(document.Name, document.Name)
		index.AddExtensionTerms(document.Name, normalizedExtension(filepath.Ext(document.Name)))
	}
	return index, tokensByDoc
}

// addTermsFromContent runs the second pass: adds BM25 terms from document content tokens.
func addTermsFromContent(index *domain.InvertedIndex, tokensByDoc map[string][]domain.TokenWithPosition) {
	for docName, tokens := range tokensByDoc {
		frequencies, positions := domain.CountTokenFrequencies(tokens)
		for term, freq := range frequencies {
			index.AddTerm(term, docName, freq, positions[term])
		}
	}
}

// addFileNameTermsToIndex runs the third pass: indexes filename tokens for recall.
// Only adds terms not already present in content so content-based scores are not distorted.
func addFileNameTermsToIndex(index *domain.InvertedIndex, documents []domain.IndexDocument) {
	for _, document := range documents {
		fileNameTokens := domain.TokenizeFileName(document.Name)
		freqs, positions := domain.CountTokenFrequencies(fileNameTokens)
		for term, freq := range freqs {
			if termStats := index.Terms[term]; termStats != nil {
				if _, alreadyIndexed := termStats.Docs[document.Name]; alreadyIndexed {
					continue
				}
			}
			index.AddTerm(term, document.Name, freq, positions[term])
		}
	}
}

func normalizedExtension(extension string) string {
	trimmed := strings.TrimSpace(extension)
	if trimmed == "" {
		return ""
	}

	return strings.TrimPrefix(strings.ToLower(trimmed), ".")
}
