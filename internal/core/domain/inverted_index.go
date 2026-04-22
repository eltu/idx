package domain

import "math"

// InvertedIndex represents a BM25 inverted index for a directory of files.
// It maps terms to documents and frequencies for relevance scoring.
type InvertedIndex struct {
	DocumentCount    int                    `json:"documentCount"`
	AverageDocLength float64                `json:"averageDocLength"`
	Terms            map[string]*TermStats  `json:"terms"`
	Documents        map[string]*DocStats   `json:"documents"`
}

// TermStats holds statistical data for a single term across all documents.
type TermStats struct {
	IDF  float64                `json:"idf"`  // Inverse document frequency
	Docs map[string]*DocTermStats `json:"docs"` // docName -> term frequency in that doc
}

// DocTermStats tracks term occurrences in a specific document.
type DocTermStats struct {
	TF        int   `json:"tf"`        // Term frequency (how many times this term appears)
	Positions []int `json:"positions"` // Character positions where term starts
}

// DocStats holds document-level statistics for BM25 calculation.
type DocStats struct {
	Length int `json:"length"` // Number of tokens in document
}

// NewInvertedIndex creates an empty inverted index.
func NewInvertedIndex() *InvertedIndex {
	return &InvertedIndex{
		DocumentCount:    0,
		AverageDocLength: 0,
		Terms:            make(map[string]*TermStats),
		Documents:        make(map[string]*DocStats),
	}
}

// AddDocument registers a new document in the index.
func (idx *InvertedIndex) AddDocument(docName string, tokenCount int) {
	idx.Documents[docName] = &DocStats{Length: tokenCount}
	idx.DocumentCount++
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
	return math.Log(float64(docCount-docFreq+1) / float64(docFreq+1))
}
