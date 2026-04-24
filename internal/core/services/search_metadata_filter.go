package services

import (
	"idx/internal/core/domain"
	"idx/internal/core/ports"
	"strings"
)

func metadataMatchedDocuments(index *domain.InvertedIndex, options ports.SearchOptions) map[string]struct{} {
	matched := allIndexedDocuments(index)
	matched = intersectMetadataPatterns(matched, index.FileNameTerms, options.FileQueries)
	matched = intersectMetadataPatterns(matched, index.PathTerms, options.PathQueries)
	return matched
}

func allIndexedDocuments(index *domain.InvertedIndex) map[string]struct{} {
	documents := make(map[string]struct{}, len(index.Documents))
	for docName := range index.Documents {
		documents[docName] = struct{}{}
	}
	return documents
}

func intersectMetadataPatterns(current map[string]struct{}, index map[string]map[string]bool, patterns []string) map[string]struct{} {
	if len(patterns) == 0 {
		return current
	}

	matchedByAnyPattern := make(map[string]struct{})
	for _, pattern := range patterns {
		for docName := range matchedMetadataPatternDocuments(index, pattern) {
			matchedByAnyPattern[docName] = struct{}{}
		}
	}

	if len(matchedByAnyPattern) == 0 {
		return map[string]struct{}{}
	}

	for docName := range current {
		if _, ok := matchedByAnyPattern[docName]; ok {
			continue
		}

		delete(current, docName)
	}

	return current
}

func matchedMetadataPatternDocuments(index map[string]map[string]bool, pattern string) map[string]bool {
	terms := metadataFilterTerms(pattern)
	if len(terms) == 0 {
		return map[string]bool{}
	}

	matched := make(map[string]bool)
	initialized := false
	for _, term := range terms {
		documents := matchedMetadataDocuments(index, term)
		if len(documents) == 0 {
			return map[string]bool{}
		}

		if !initialized {
			for docName := range documents {
				matched[docName] = true
			}
			initialized = true
			continue
		}

		for docName := range matched {
			if documents[docName] {
				continue
			}

			delete(matched, docName)
		}
	}

	return matched
}

func matchedMetadataDocuments(index map[string]map[string]bool, term string) map[string]bool {
	if !strings.Contains(term, "*") {
		return index[term]
	}

	matched := make(map[string]bool)
	for indexedTerm, docs := range index {
		if !wildcardMatch(term, indexedTerm) {
			continue
		}

		for docName := range docs {
			matched[docName] = true
		}
	}

	return matched
}

func wildcardMatch(pattern string, value string) bool {
	if pattern == "*" {
		return true
	}

	if strings.Count(pattern, "*") == 1 {
		if strings.HasSuffix(pattern, "*") {
			return strings.HasPrefix(value, strings.TrimSuffix(pattern, "*"))
		}

		if strings.HasPrefix(pattern, "*") {
			return strings.HasSuffix(value, strings.TrimPrefix(pattern, "*"))
		}
	}

	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return value == pattern
	}

	position := 0
	for index, part := range parts {
		if part == "" {
			continue
		}

		offset := strings.Index(value[position:], part)
		if offset < 0 {
			return false
		}

		start := position + offset
		if index == 0 && !strings.HasPrefix(pattern, "*") && start != 0 {
			return false
		}

		position = start + len(part)
	}

	if strings.HasSuffix(pattern, "*") {
		return true
	}

	lastPart := parts[len(parts)-1]
	if lastPart == "" {
		return true
	}

	return strings.HasSuffix(value, lastPart)
}

func metadataFilterTerms(query string) []string {
	lower := strings.ToLower(query)
	terms := make([]string, 0)
	seen := make(map[string]struct{})
	for _, term := range splitMetadataFilterTerms(lower) {
		if _, exists := seen[term]; exists {
			continue
		}

		seen[term] = struct{}{}
		terms = append(terms, term)
	}

	return terms
}

func splitMetadataFilterTerms(query string) []string {
	terms := make([]string, 0)
	var builder strings.Builder
	flush := func() {
		if builder.Len() == 0 {
			return
		}

		terms = append(terms, builder.String())
		builder.Reset()
	}

	for index := 0; index < len(query); index++ {
		char := query[index]
		if isMetadataFilterChar(char) {
			builder.WriteByte(char)
			continue
		}

		flush()
	}

	flush()
	return terms
}

func isMetadataFilterChar(char byte) bool {
	return char == '*' ||
		(char >= 'a' && char <= 'z') ||
		(char >= '0' && char <= '9') ||
		char == '_'
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
