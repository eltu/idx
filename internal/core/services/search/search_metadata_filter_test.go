package search

import (
	"testing"

	"idx/internal/core/domain"
	"idx/internal/core/ports"
)

func TestWildcardMatchPatterns(t *testing.T) {
	testCases := []struct {
		name     string
		pattern  string
		value    string
		expected bool
	}{
		{name: "match all", pattern: "*", value: "anything", expected: true},
		{name: "exact match", pattern: "core", value: "core", expected: true},
		{name: "exact mismatch", pattern: "core", value: "module", expected: false},
		{name: "prefix wildcard", pattern: "mod*", value: "module", expected: true},
		{name: "suffix wildcard", pattern: "*ule", value: "module", expected: true},
		{name: "middle wildcard", pattern: "m*le", value: "module", expected: true},
		{name: "anchored prefix fail", pattern: "mod*le", value: "xmodule", expected: false},
		{name: "anchored suffix fail", pattern: "mod*le", value: "modulex", expected: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			matched := wildcardMatch(testCase.pattern, testCase.value)
			if matched != testCase.expected {
				t.Fatalf("wildcardMatch(%q, %q) = %v, expected %v", testCase.pattern, testCase.value, matched, testCase.expected)
			}
		})
	}
}

func TestMatchedMetadataPatternDocumentsIntersectsPatternTerms(t *testing.T) {
	index := map[string]map[string]bool{
		"internal": {"doc1": true, "doc2": true},
		"core":     {"doc1": true},
		"services": {"doc2": true},
	}

	matched := matchedMetadataPatternDocuments(index, "internal core")
	if len(matched) != 1 || !matched["doc1"] {
		t.Fatalf("expected only doc1 for pattern intersection, got %v", matched)
	}

	none := matchedMetadataPatternDocuments(index, "internal unknown")
	if len(none) != 0 {
		t.Fatalf("expected no matches for missing term, got %v", none)
	}
}

func TestIntersectMetadataPatternsUsesUnionAcrossPatterns(t *testing.T) {
	current := map[string]struct{}{"doc1": {}, "doc2": {}, "doc3": {}}
	index := map[string]map[string]bool{
		"internal": {"doc1": true},
		"docs":     {"doc2": true},
	}

	filtered := intersectMetadataPatterns(current, index, []string{"internal", "docs"})
	if len(filtered) != 2 {
		t.Fatalf("expected two documents after union/intersection, got %v", filtered)
	}
	if _, ok := filtered["doc1"]; !ok {
		t.Fatalf("expected doc1 to remain, got %v", filtered)
	}
	if _, ok := filtered["doc2"]; !ok {
		t.Fatalf("expected doc2 to remain, got %v", filtered)
	}
	if _, ok := filtered["doc3"]; ok {
		t.Fatalf("expected doc3 to be removed, got %v", filtered)
	}
}

func TestApplyMetadataFilterFallsBackToDocumentPathMatch(t *testing.T) {
	current := map[string]struct{}{"doc1": {}, "doc2": {}}
	termIndex := map[string]map[string]bool{}
	documents := map[string]*domain.DocStats{
		"doc1": {Path: "internal/core/app.go"},
		"doc2": {Path: "docs/readme.md"},
	}

	matched := applyMetadataFilter(current, termIndex, documents, []string{"internal core"}, pathMetadataValue)
	if len(matched) != 1 {
		t.Fatalf("expected one fallback match, got %v", matched)
	}
	if _, ok := matched["doc1"]; !ok {
		t.Fatalf("expected doc1 fallback match, got %v", matched)
	}
}

func TestMetadataMatchedDocumentsFiltersByExtension(t *testing.T) {
	index := domain.NewInvertedIndex()
	index.Documents["main.go"] = &domain.DocStats{Name: "main.go", Path: "cmd/main.go", Length: 3}
	index.Documents["README.md"] = &domain.DocStats{Name: "README.md", Path: "README.md", Length: 3}
	index.AddPathTerms("main.go", "cmd/main.go")
	index.AddPathTerms("README.md", "README.md")
	index.AddExtensionTerms("main.go", "go")
	index.AddExtensionTerms("README.md", "md")

	matched := metadataMatchedDocuments(index, ports.SearchOptions{ExtensionQuery: "go"})
	if len(matched) != 1 {
		t.Fatalf("expected one .go document, got %v", matched)
	}
	if _, ok := matched["main.go"]; !ok {
		t.Fatalf("expected main.go to match extension filter, got %v", matched)
	}
}

func TestEffectiveExtensionPatternsNormalizesDotAndCase(t *testing.T) {
	patterns := effectiveExtensionPatterns([]string{".GO", " md "}, "")
	if len(patterns) != 2 {
		t.Fatalf("expected 2 normalized patterns, got %v", patterns)
	}
	if patterns[0] != "go" || patterns[1] != "md" {
		t.Fatalf("expected normalized extensions [go md], got %v", patterns)
	}
}
