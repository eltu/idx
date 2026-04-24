package domain

import (
	"strings"
)

// TokenWithPosition represents a term and its position in text.
type TokenWithPosition struct {
	Token    string
	Position int
}

// TokenizeText extracts tokens from text with their positions.
// Returns lowercase tokens split only by whitespace.
func TokenizeText(text string) []TokenWithPosition {
	fields := strings.Fields(text)
	tokens := make([]TokenWithPosition, 0, len(fields))
	searchFrom := 0

	for _, field := range fields {
		position := strings.Index(text[searchFrom:], field)
		if position < 0 {
			continue
		}

		absolutePos := searchFrom + position
		tokens = append(tokens, TokenWithPosition{
			Token:    strings.ToLower(field),
			Position: absolutePos,
		})

		searchFrom = absolutePos + len(field)
	}

	return tokens
}

// CountTokenFrequencies returns a map of token->count and token->positions.
func CountTokenFrequencies(tokens []TokenWithPosition) (map[string]int, map[string][]int) {
	frequencies := make(map[string]int)
	positions := make(map[string][]int)

	for _, token := range tokens {
		frequencies[token.Token]++
		positions[token.Token] = append(positions[token.Token], token.Position)
	}

	return frequencies, positions
}

// BM25Score computes BM25 relevance score for a term in a document.
// Parameters:
//   - tf: term frequency in document
//   - idf: inverse document frequency
//   - docLength: number of tokens in document
//   - avgDocLength: average document length in corpus
//   - k1: saturation parameter (default: 1.5)
//   - b: length normalization parameter (default: 0.75)
func BM25Score(tf int, idf float64, docLength int, avgDocLength float64, k1 float64, b float64) float64 {
	if idf == 0 {
		return 0
	}

	numerator := float64(tf) * (k1 + 1)
	denominator := float64(tf) + k1*(1-b+b*float64(docLength)/avgDocLength)

	return idf * (numerator / denominator)
}
