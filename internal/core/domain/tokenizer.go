package domain

import (
	"strings"
	"unicode"
)

// TokenWithPosition represents a term and its position in text.
type TokenWithPosition struct {
	Token    string
	Position int
}

// TokenizeText extracts tokens from text with their positions.
// Returns lowercase tokens split by standard delimiters.
// Periods are kept only inside terms (e.g. versions/domains) and hyphens
// are kept only for alphanumeric product-like terms that include digits.
func TokenizeText(text string) []TokenWithPosition {
	tokens := make([]TokenWithPosition, 0)
	for index := 0; index < len(text); {
		if !isTokenChar(text, index) {
			index++
			continue
		}

		start := index
		for index < len(text) && isTokenChar(text, index) {
			index++
		}

		token := strings.ToLower(text[start:index])
		if token == "" {
			continue
		}

		tokens = append(tokens, TokenWithPosition{Token: token, Position: start})
	}

	return tokens
}

func isTokenChar(text string, index int) bool {
	char := text[index]
	if isAlphaNumeric(char) || char == '_' {
		return true
	}

	if char == '.' {
		return isInnerPeriod(text, index)
	}

	if char == '-' {
		return isNumericHyphen(text, index)
	}

	return false
}

func isAlphaNumeric(char byte) bool {
	if char >= 'a' && char <= 'z' {
		return true
	}

	if char >= 'A' && char <= 'Z' {
		return true
	}

	return char >= '0' && char <= '9'
}

func isInnerPeriod(text string, index int) bool {
	if index == 0 || index == len(text)-1 {
		return false
	}

	return isAlphaNumeric(text[index-1]) && isAlphaNumeric(text[index+1])
}

func isNumericHyphen(text string, index int) bool {
	if index == 0 || index == len(text)-1 {
		return false
	}

	left := text[index-1]
	right := text[index+1]
	if !isAlphaNumeric(left) || !isAlphaNumeric(right) {
		return false
	}

	return isDigit(left) || isDigit(right)
}

func isDigit(char byte) bool {
	return char >= '0' && char <= '9'
}

// TokenizeFileName extracts tokens from a file name, splitting on '_', '.', '-',
// and CamelCase word boundaries. Returns lowercase tokens.
// Positions are set to 0 because filename tokens are not content positions.
//
// Examples:
//
//	TokenizeFileName("main_test.go")     → [{main,0},{test,0},{go,0}]
//	TokenizeFileName("InvertedIndex.go") → [{inverted,0},{index,0},{go,0}]
//	TokenizeFileName("bm25_score.go")    → [{bm25,0},{score,0},{go,0}]
func TokenizeFileName(fileName string) []TokenWithPosition {
	parts := strings.FieldsFunc(fileName, func(r rune) bool {
		return r == '_' || r == '.' || r == '-' || r == '/'
	})

	tokens := make([]TokenWithPosition, 0, len(parts)*2)
	for _, part := range parts {
		for _, word := range splitCamelCaseWords([]rune(part)) {
			lower := strings.ToLower(word)
			if lower != "" {
				tokens = append(tokens, TokenWithPosition{Token: lower, Position: 0})
			}
		}
	}

	return tokens
}

// splitCamelCaseWords splits a rune slice on CamelCase boundaries.
// "InvertedIndex" → ["Inverted", "Index"]
// "bm25Score"     → ["bm25", "Score"]
func splitCamelCaseWords(runes []rune) []string {
	if len(runes) == 0 {
		return nil
	}

	words := []string{}
	start := 0
	for i := 1; i < len(runes); i++ {
		if unicode.IsUpper(runes[i]) && unicode.IsLower(runes[i-1]) {
			words = append(words, string(runes[start:i]))
			start = i
		}
	}

	words = append(words, string(runes[start:]))
	return words
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
