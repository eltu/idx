package services

import (
	"idx/internal/core/domain"
	"idx/internal/core/ports"
)

func metadataMatchedDocuments(index *domain.InvertedIndex, options ports.SearchOptions) map[string]struct{} {
	matched := allIndexedDocuments(index)
	matched = intersectMetadataTerms(matched, index.FileNameTerms, uniqueQueryTerms(options.FileQuery))
	matched = intersectMetadataTerms(matched, index.PathTerms, uniqueQueryTerms(options.PathQuery))
	return matched
}

func allIndexedDocuments(index *domain.InvertedIndex) map[string]struct{} {
	documents := make(map[string]struct{}, len(index.Documents))
	for docName := range index.Documents {
		documents[docName] = struct{}{}
	}
	return documents
}

func intersectMetadataTerms(current map[string]struct{}, index map[string]map[string]bool, terms []string) map[string]struct{} {
	if len(terms) == 0 {
		return current
	}

	for _, term := range terms {
		documents := index[term]
		for docName := range current {
			if documents[docName] {
				continue
			}

			delete(current, docName)
		}
	}

	return current
}

func filteredScores(scores map[string]float64, metadataMatches map[string]struct{}, metadataOnly bool) map[string]float64 {
	if metadataOnly {
		return uniformScores(metadataMatches)
	}

	filtered := make(map[string]float64)
	for docName, score := range scores {
		if _, ok := metadataMatches[docName]; !ok {
			continue
		}

		filtered[docName] = score
	}

	return filtered
}

func uniformScores(documents map[string]struct{}) map[string]float64 {
	scores := make(map[string]float64, len(documents))
	for docName := range documents {
		scores[docName] = 1.0
	}

	return scores
}
