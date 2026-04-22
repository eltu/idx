package domain

import (
	"regexp"
	"strings"
)

// StopWords contains common English words to exclude from indexing.
var StopWords = map[string]bool{
	"the":  true,
	"a":    true,
	"an":   true,
	"and":  true,
	"or":   true,
	"but":  true,
	"in":   true,
	"on":   true,
	"at":   true,
	"to":   true,
	"for":  true,
	"of":   true,
	"with": true,
	"by":   true,
	"from": true,
	"is":   true,
	"are":  true,
	"was":  true,
	"be":   true,
	"that": true,
	"this": true,
	"it":   true,
	"as":   true,
	"if":   true,
	"so":   true,
}

// TokenWithPosition represents a term and its position in text.
type TokenWithPosition struct {
	Token    string
	Position int
}

// TokenizeText extracts tokens from text with their positions.
// Returns tokens in lowercase, filtering stop words and short tokens.
func TokenizeText(text string) []TokenWithPosition {
	// Replace non-alphanumeric characters with spaces
	reg := regexp.MustCompile(`[^a-zA-Z0-9_\s]`)
	cleanText := reg.ReplaceAllString(text, " ")

	var tokens []TokenWithPosition
	words := strings.Fields(cleanText)
	position := 0

	for _, word := range words {
		lower := strings.ToLower(word)

		// Skip stop words and very short tokens
		if len(lower) < 2 || StopWords[lower] {
			position += len(word) + 1
			continue
		}

		tokens = append(tokens, TokenWithPosition{
			Token:    lower,
			Position: position,
		})

		position += len(word) + 1
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
