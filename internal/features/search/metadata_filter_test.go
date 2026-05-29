package search

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"idx/internal/features/indexing"
)

func TestWildcardMatch_Patterns(t *testing.T) {
	t.Parallel()

	tests := []struct {
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

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Act
			matched := wildcardMatch(tc.pattern, tc.value)

			// Assert
			assert.Equal(t, tc.expected, matched, "wildcardMatch(%q, %q)", tc.pattern, tc.value)
		})
	}
}

func TestMatchedMetadataPatternDocuments_IntersectsPatternTerms(t *testing.T) {
	t.Parallel()

	// Arrange
	index := map[string]map[string]bool{
		"internal": {"doc1": true, "doc2": true},
		"core":     {"doc1": true},
		"services": {"doc2": true},
	}

	// Act
	matched := matchedMetadataPatternDocuments(index, "internal core")
	none := matchedMetadataPatternDocuments(index, "internal unknown")

	// Assert
	require.Len(t, matched, 1)
	assert.True(t, matched["doc1"])
	assert.Empty(t, none)
}

func TestIntersectMetadataPatterns_UsesUnionAcrossPatterns(t *testing.T) {
	t.Parallel()

	// Arrange
	current := map[string]struct{}{"doc1": {}, "doc2": {}, "doc3": {}}
	index := map[string]map[string]bool{
		"internal": {"doc1": true},
		"docs":     {"doc2": true},
	}

	// Act
	filtered := intersectMetadataPatterns(current, index, []string{"internal", "docs"})

	// Assert
	assert.Len(t, filtered, 2)
	assert.Contains(t, filtered, "doc1")
	assert.Contains(t, filtered, "doc2")
	assert.NotContains(t, filtered, "doc3")
}

func TestApplyMetadataFilter_FallsBackToDocumentPathMatch(t *testing.T) {
	t.Parallel()

	// Arrange
	current := map[string]struct{}{"doc1": {}, "doc2": {}}
	termIndex := map[string]map[string]bool{}
	documents := map[string]*indexing.DocStats{
		"doc1": {Path: "internal/core/app.go"},
		"doc2": {Path: "docs/readme.md"},
	}

	// Act
	matched := applyMetadataFilter(current, termIndex, documents, []string{"internal core"}, pathMetadataValue)

	// Assert
	require.Len(t, matched, 1)
	assert.Contains(t, matched, "doc1")
}

func TestMetadataMatchedDocuments_FiltersByExtension(t *testing.T) {
	t.Parallel()

	// Arrange
	index := indexing.NewInvertedIndex()
	index.Documents["main.go"] = &indexing.DocStats{Name: "main.go", Path: "cmd/main.go", Length: 3}
	index.Documents["README.md"] = &indexing.DocStats{Name: "README.md", Path: "README.md", Length: 3}
	index.AddPathTerms("main.go", "cmd/main.go")
	index.AddPathTerms("README.md", "README.md")
	index.AddExtensionTerms("main.go", "go")
	index.AddExtensionTerms("README.md", "md")

	// Act
	matched := metadataMatchedDocuments(index, Options{ExtensionQuery: "go"})

	// Assert
	require.Len(t, matched, 1)
	assert.Contains(t, matched, "main.go")
}

func TestApplyMetadataFilter_FallsBackToExtensionMatch(t *testing.T) {
	t.Parallel()

	// Arrange — empty termIndex forces the fallback path, exercising extensionMetadataValue
	current := map[string]struct{}{"app.go": {}, "readme.md": {}}
	termIndex := map[string]map[string]bool{}
	documents := map[string]*indexing.DocStats{
		"app.go":    {Path: "cmd/app.go"},
		"readme.md": {Path: "README.md"},
	}

	// Act
	matched := applyMetadataFilter(current, termIndex, documents, []string{"go"}, extensionMetadataValue)

	// Assert
	require.Len(t, matched, 1)
	assert.Contains(t, matched, "app.go")
}

func TestEffectiveExtensionPatterns_NormalizesDotAndCase(t *testing.T) {
	t.Parallel()

	// Act
	patterns := effectiveExtensionPatterns([]string{".GO", " md "}, "")

	// Assert
	require.Len(t, patterns, 2)
	assert.Equal(t, "go", patterns[0])
	assert.Equal(t, "md", patterns[1])
}
