package domain

import "math"

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
