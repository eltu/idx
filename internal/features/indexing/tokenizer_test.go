package indexing

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenizeText_Scenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "StandardDelimiters",
			input:    "fmt.Println(\"Hello\") if x == y { return a+b }",
			expected: []string{"fmt.println", "hello", "if", "x", "y", "return", "a", "b"},
		},
		{
			name:     "PeriodsByContext",
			input:    "version 1.26.1. domain example.com. trailing.",
			expected: []string{"version", "1.26.1", "domain", "example.com", "trailing"},
		},
		{
			name:     "HyphenSplitUnlessNumeric",
			input:    "foo-bar F-150 alpha-beta x-1 2024-10",
			expected: []string{"foo", "bar", "f-150", "alpha", "beta", "x-1", "2024-10"},
		},
		{
			name:     "DoesNotRemoveStopwords",
			input:    "the and if to",
			expected: []string{"the", "and", "if", "to"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Act
			tokens := TokenizeText(tc.input)

			// Assert
			require.Len(t, tokens, len(tc.expected))
			for i, tok := range tokens {
				assert.Equal(t, tc.expected[i], tok.Token, "token[%d]", i)
			}
		})
	}
}

func TestTokenizeText_TracksTokenPositions(t *testing.T) {
	t.Parallel()

	// Arrange
	text := "go\tfmt.Println\nx"
	expectedTokens := []string{"go", "fmt.println", "x"}
	expectedPositions := []int{0, 3, 15}

	// Act
	tokens := TokenizeText(text)

	// Assert
	require.Len(t, tokens, len(expectedTokens))
	for i, tok := range tokens {
		assert.Equal(t, expectedTokens[i], tok.Token, "token[%d]", i)
		assert.Equal(t, expectedPositions[i], tok.Position, "position[%d]", i)
	}
}

func TestTokenizeFileName_Scenarios(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "SnakeCase",
			input:    "main_test.go",
			expected: []string{"main", "test", "go"},
		},
		{
			name:     "CamelCase",
			input:    "InvertedIndex.go",
			expected: []string{"inverted", "index", "go"},
		},
		{
			name:     "Mixed",
			input:    "bm25_score.go",
			expected: []string{"bm25", "score", "go"},
		},
		{
			name:     "NoExtension_SplitsOnUnderscore",
			input:    "search_service",
			expected: []string{"search", "service"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Act
			tokens := tokenStrings(TokenizeFileName(tc.input))

			// Assert
			assertTokensEqual(t, tokens, tc.expected)
		})
	}
}

func TestSplitCamelCaseWords_EmptyInput_ReturnsNil(t *testing.T) {
	t.Parallel()

	// Act
	result := splitCamelCaseWords([]rune{})

	// Assert
	assert.Nil(t, result)
}

func TestIsNumericHyphen_NonAlphanumericAdjacent_ReturnsFalse(t *testing.T) {
	t.Parallel()

	// "-" followed by " " (space) — non-alphanumeric right side → false
	assert.False(t, isNumericHyphen("a- b", 1))
}

func TestIsNumericHyphen_AtBoundary_ReturnsFalse(t *testing.T) {
	t.Parallel()

	assert.False(t, isNumericHyphen("-ab", 0), "hyphen at index 0")
	assert.False(t, isNumericHyphen("ab-", 2), "hyphen at last index")
}

func tokenStrings(twp []TokenWithPosition) []string {
	out := make([]string, len(twp))
	for i, t := range twp {
		out[i] = t.Token
	}
	return out
}

func assertTokensEqual(t *testing.T, got, want []string) {
	t.Helper()
	require.Len(t, got, len(want), "token count mismatch: got %v, want %v", got, want)
	for i, tok := range got {
		assert.Equal(t, want[i], tok, "token[%d]", i)
	}
}
