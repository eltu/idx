package indexing

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvertedIndex_DocumentAndTermLifecycle(t *testing.T) {
	t.Parallel()

	// Arrange
	index := NewInvertedIndex()

	// Act
	index.AddDocument("a.txt", "repo/a.txt", 3)
	index.AddDocument("b.txt", "repo/b.txt", 1)
	index.AddTerm("go", "a.txt", 2, []int{0, 5})
	index.AddTerm("go", "b.txt", 1, []int{2})
	index.AddFileNameTerms("a.txt", "a.txt")
	index.AddPathTerms("a.txt", "repo/docs/a.txt")
	index.AddExtensionTerms("a.txt", "txt")
	index.CalculateAverageDocLen()
	index.CalculateIDF()

	// Assert
	assert.Equal(t, 2, index.DocumentCount)
	assert.Equal(t, float64(2), index.AverageDocLength)
	require.NotNil(t, index.Terms["go"])
	assert.Len(t, index.Terms["go"].Docs, 2)
	assert.Positive(t, index.Terms["go"].IDF)
	assert.True(t, index.FileNameTerms["a.txt"]["a.txt"], "expected filename token indexed")
	assert.True(t, index.PathTerms["repo"]["a.txt"], "expected path segment 'repo' indexed")
	assert.True(t, index.PathTerms["docs"]["a.txt"], "expected path segment 'docs' indexed")
	assert.True(t, index.PathTerms["a.txt"]["a.txt"], "expected path segment 'a.txt' indexed")
	assert.True(t, index.ExtensionTerms["txt"]["a.txt"], "expected extension 'txt' indexed")
}

func TestInvertedIndex_CalculateAverageDocLen_ZeroWithNoDocuments(t *testing.T) {
	t.Parallel()

	// Arrange
	index := NewInvertedIndex()

	// Act
	index.CalculateAverageDocLen()

	// Assert
	assert.Equal(t, float64(0), index.AverageDocLength)
}

func TestIDFScore_AndBM25Score_EdgeCases(t *testing.T) {
	t.Parallel()

	// Assert — idf with zero df returns zero
	assert.Equal(t, float64(0), idfScore(10, 0))

	// Assert — idf with valid df returns positive
	idf := idfScore(10, 2)
	assert.Positive(t, idf)

	// Assert — BM25 with positive idf returns positive
	score := BM25Score(3, idf, 100, 80, 1.5, 0.75)
	assert.Positive(t, score)
	assert.False(t, math.IsNaN(score))
	assert.False(t, math.IsInf(score, 0))

	// Assert — BM25 with zero idf returns zero
	assert.Equal(t, float64(0), BM25Score(3, 0, 100, 80, 1.5, 0.75))
}

func TestCountTokenFrequencies_ReturnsCountsAndPositions(t *testing.T) {
	t.Parallel()

	// Arrange
	tokens := []TokenWithPosition{
		{Token: "go", Position: 0},
		{Token: "idx", Position: 3},
		{Token: "go", Position: 7},
	}

	// Act
	freq, pos := CountTokenFrequencies(tokens)

	// Assert
	assert.Equal(t, 2, freq["go"])
	assert.Equal(t, 1, freq["idx"])
	require.Len(t, pos["go"], 2)
	assert.Equal(t, 0, pos["go"][0])
	assert.Equal(t, 7, pos["go"][1])
}
