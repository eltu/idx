package search

import (
	"path/filepath"
	"strings"

	"idx/internal/features/indexing"
	
)

func metadataMatchedDocuments(index *indexing.InvertedIndex, options Options) map[string]struct{} {
	matched := allIndexedDocuments(index)
	pathPatterns := effectiveMetadataPatterns(options.PathQueries, options.PathQuery)
	matched = applyMetadataFilter(matched, index.PathTerms, index.Documents, pathPatterns, pathMetadataValue)

	extensionPatterns := effectiveExtensionPatterns(options.ExtensionQueries, options.ExtensionQuery)
	return applyMetadataFilter(matched, index.ExtensionTerms, index.Documents, extensionPatterns, extensionMetadataValue)
}

func applyMetadataFilter(current map[string]struct{}, termIndex map[string]map[string]bool, documents map[string]*indexing.DocStats, patterns []string, metadataValue metadataValueResolver) map[string]struct{} {
	if len(patterns) == 0 {
		return current
	}

	indexed := intersectMetadataPatterns(cloneDocSet(current), termIndex, patterns)
	if len(indexed) > 0 {
		return indexed
	}

	return fallbackMetadataPatternMatch(current, documents, patterns, metadataValue)
}

type metadataValueResolver func(document *indexing.DocStats) string

func cloneDocSet(current map[string]struct{}) map[string]struct{} {
	clone := make(map[string]struct{}, len(current))
	for docName := range current {
		clone[docName] = struct{}{}
	}

	return clone
}

func fallbackMetadataPatternMatch(current map[string]struct{}, documents map[string]*indexing.DocStats, patterns []string, metadataValue metadataValueResolver) map[string]struct{} {
	matched := make(map[string]struct{})
	for docName := range current {
		document := documents[docName]
		if document == nil {
			continue
		}

		if metadataValueMatchesAnyPattern(metadataValue(document), patterns) {
			matched[docName] = struct{}{}
		}
	}

	return matched
}

func metadataValueMatchesAnyPattern(value string, patterns []string) bool {
	valueTerms := splitMetadataFilterTerms(strings.ToLower(value))
	for _, pattern := range patterns {
		if metadataValueMatchesPattern(valueTerms, pattern) {
			return true
		}
	}

	return false
}

func metadataValueMatchesPattern(valueTerms []string, pattern string) bool {
	patternTerms := metadataFilterTerms(pattern)
	if len(patternTerms) == 0 {
		return false
	}

	for _, patternTerm := range patternTerms {
		if !hasMatchingMetadataTerm(valueTerms, patternTerm) {
			return false
		}
	}

	return true
}

func hasMatchingMetadataTerm(valueTerms []string, patternTerm string) bool {
	for _, valueTerm := range valueTerms {
		if wildcardMatch(patternTerm, valueTerm) {
			return true
		}
	}

	return false
}

func effectiveMetadataPatterns(patterns []string, fallback string) []string {
	if len(patterns) > 0 {
		return patterns
	}

	trimmed := strings.TrimSpace(fallback)
	if trimmed == "" {
		return []string{}
	}

	return []string{trimmed}
}

func effectiveExtensionPatterns(patterns []string, fallback string) []string {
	normalized := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		if extension := normalizeExtensionPattern(pattern); extension != "" {
			normalized = append(normalized, extension)
		}
	}

	if len(normalized) > 0 {
		return normalized
	}

	extension := normalizeExtensionPattern(fallback)
	if extension == "" {
		return []string{}
	}

	return []string{extension}
}

func normalizeExtensionPattern(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return ""
	}

	if strings.HasPrefix(trimmed, ".") {
		return strings.TrimPrefix(trimmed, ".")
	}

	return trimmed
}

func pathMetadataValue(document *indexing.DocStats) string {
	return document.Path
}

func extensionMetadataValue(document *indexing.DocStats) string {
	return normalizeExtensionPattern(filepath.Ext(document.Path))
}

func allIndexedDocuments(index *indexing.InvertedIndex) map[string]struct{} {
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
	matched, ok := initialDocumentSet(index, terms[0])
	if !ok {
		return map[string]bool{}
	}
	for _, term := range terms[1:] {
		documents := matchedMetadataDocuments(index, term)
		if len(documents) == 0 {
			return map[string]bool{}
		}
		retainIntersection(matched, documents)
	}
	return matched
}

func initialDocumentSet(index map[string]map[string]bool, term string) (map[string]bool, bool) {
	documents := matchedMetadataDocuments(index, term)
	if len(documents) == 0 {
		return nil, false
	}
	matched := make(map[string]bool, len(documents))
	for docName := range documents {
		matched[docName] = true
	}
	return matched, true
}

func retainIntersection(matched map[string]bool, documents map[string]bool) {
	for docName := range matched {
		if !documents[docName] {
			delete(matched, docName)
		}
	}
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

	if singleWildcardMatch(pattern, value) {
		return true
	}

	return wildcardPartsMatch(pattern, value)
}

func singleWildcardMatch(pattern string, value string) bool {
	if strings.Count(pattern, "*") != 1 {
		return false
	}

	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(value, strings.TrimSuffix(pattern, "*"))
	}

	if strings.HasPrefix(pattern, "*") {
		return strings.HasSuffix(value, strings.TrimPrefix(pattern, "*"))
	}

	return false
}

func wildcardPartsMatch(pattern string, value string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return value == pattern
	}

	if !matchesWildcardParts(pattern, value, parts) {
		return false
	}

	return matchesWildcardSuffix(pattern, value, parts)
}

func matchesWildcardParts(pattern string, value string, parts []string) bool {
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

	return true
}

func matchesWildcardSuffix(pattern string, value string, parts []string) bool {
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
