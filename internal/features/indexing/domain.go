package indexing

import (
	"math"
	"strings"
	"unicode"
)

// InvertedIndex represents a BM25 inverted index for a directory of files.
// It maps terms to documents and frequencies for relevance scoring.
type InvertedIndex struct {
	DocumentCount    int                        `json:"documentCount"`
	AverageDocLength float64                    `json:"averageDocLength"`
	Terms            map[string]*TermStats      `json:"terms"`
	FileNameTerms    map[string]map[string]bool `json:"fileNameTerms"`
	PathTerms        map[string]map[string]bool `json:"pathTerms"`
	ExtensionTerms   map[string]map[string]bool `json:"extensionTerms"`
	Documents        map[string]*DocStats       `json:"documents"`
}

// TermStats holds statistical data for a single term across all documents.
type TermStats struct {
	IDF  float64                  `json:"idf"`  // Inverse document frequency
	Docs map[string]*DocTermStats `json:"docs"` // docName -> term frequency in that doc
}

// DocTermStats tracks term occurrences in a specific document.
type DocTermStats struct {
	TF        int   `json:"tf"`        // Term frequency (how many times this term appears)
	Positions []int `json:"positions"` // Character positions where term starts
}

// DocStats holds document-level statistics for BM25 calculation.
type DocStats struct {
	Name   string `json:"name"`   // Base file name
	Path   string `json:"path"`   // File path as collected during indexing
	Length int    `json:"length"` // Number of tokens in document
}

// IndexDocument describes a file to be indexed.
type IndexDocument struct {
	Name    string
	Path    string
	Content string
}

// TokenWithPosition represents a term and its position in text.
type TokenWithPosition struct {
	Token    string
	Position int
}

// NewInvertedIndex creates an empty inverted index.
func NewInvertedIndex() *InvertedIndex {
	return &InvertedIndex{
		DocumentCount:    0,
		AverageDocLength: 0,
		Terms:            make(map[string]*TermStats),
		FileNameTerms:    make(map[string]map[string]bool),
		PathTerms:        make(map[string]map[string]bool),
		ExtensionTerms:   make(map[string]map[string]bool),
		Documents:        make(map[string]*DocStats),
	}
}

// AddDocument registers a new document in the index.
func (idx *InvertedIndex) AddDocument(docName string, docPath string, tokenCount int) {
	idx.Documents[docName] = &DocStats{Name: docName, Path: docPath, Length: tokenCount}
	idx.DocumentCount++
}

// AddFileNameTerms indexes filename tokens for filter-only lookups.
func (idx *InvertedIndex) AddFileNameTerms(docName string, fileName string) {
	addMetadataTerms(idx.FileNameTerms, docName, fileName)
}

// AddPathTerms indexes path tokens for filter-only lookups.
func (idx *InvertedIndex) AddPathTerms(docName string, docPath string) {
	addMetadataTerms(idx.PathTerms, docName, docPath)
}

// AddExtensionTerms indexes extension tokens for filter-only lookups.
func (idx *InvertedIndex) AddExtensionTerms(docName string, extension string) {
	addMetadataTerms(idx.ExtensionTerms, docName, extension)
}

// AddTerm adds a term occurrence to a document in the index.
func (idx *InvertedIndex) AddTerm(term string, docName string, frequency int, positions []int) {
	if idx.Terms[term] == nil {
		idx.Terms[term] = &TermStats{
			Docs: make(map[string]*DocTermStats),
		}
	}

	idx.Terms[term].Docs[docName] = &DocTermStats{
		TF:        frequency,
		Positions: positions,
	}
}

// CalculateAverageDocLen updates the average document length after all documents are added.
func (idx *InvertedIndex) CalculateAverageDocLen() {
	if idx.DocumentCount == 0 {
		idx.AverageDocLength = 0
		return
	}

	totalLength := 0
	for _, doc := range idx.Documents {
		totalLength += doc.Length
	}

	idx.AverageDocLength = float64(totalLength) / float64(idx.DocumentCount)
}

// CalculateIDF updates BM25 IDF scores for all terms.
func (idx *InvertedIndex) CalculateIDF() {
	for _, termStats := range idx.Terms {
		docFreq := len(termStats.Docs) // Number of documents containing this term
		// IDF = log((N - df + 0.5) / (df + 0.5)) where N = document count
		termStats.IDF = idfScore(idx.DocumentCount, docFreq)
	}
}

// idfScore computes BM25 IDF using standard formula.
func idfScore(docCount int, docFreq int) float64 {
	if docFreq == 0 {
		return 0
	}

	base := (float64(docCount) - float64(docFreq) + 0.5) / (float64(docFreq) + 0.5)
	return math.Log1p(base)
}

func addMetadataTerms(index map[string]map[string]bool, docName string, text string) {
	for _, token := range TokenizeText(text) {
		documents := index[token.Token]
		if documents == nil {
			documents = make(map[string]bool)
			index[token.Token] = documents
		}

		documents[docName] = true
	}
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
