package domain

import (
	"math"
	"testing"
)

func TestInvertedIndexDocumentAndTermLifecycle(t *testing.T) {
	index := NewInvertedIndex()
	index.AddDocument("a.txt", "repo/a.txt", 3)
	index.AddDocument("b.txt", "repo/b.txt", 1)
	index.AddTerm("go", "a.txt", 2, []int{0, 5})
	index.AddTerm("go", "b.txt", 1, []int{2})
	index.AddFileNameTerms("a.txt", "a.txt")
	index.AddPathTerms("a.txt", "repo/docs/a.txt")
	index.AddExtensionTerms("a.txt", "txt")
	index.CalculateAverageDocLen()
	index.CalculateIDF()

	if index.DocumentCount != 2 {
		t.Fatalf("expected document count 2, got %d", index.DocumentCount)
	}
	if index.AverageDocLength != 2 {
		t.Fatalf("expected average doc length 2, got %v", index.AverageDocLength)
	}
	if index.Terms["go"] == nil || len(index.Terms["go"].Docs) != 2 {
		t.Fatal("expected go term in two docs")
	}
	if index.Terms["go"].IDF <= 0 {
		t.Fatalf("expected positive idf, got %f", index.Terms["go"].IDF)
	}
	if !index.FileNameTerms["a.txt"]["a.txt"] {
		t.Fatal("expected filename token to be indexed")
	}
	if !index.PathTerms["repo"]["a.txt"] || !index.PathTerms["docs"]["a.txt"] || !index.PathTerms["a.txt"]["a.txt"] {
		t.Fatal("expected path segment tokens to be indexed")
	}
	if !index.ExtensionTerms["txt"]["a.txt"] {
		t.Fatal("expected extension token to be indexed")
	}
}

func TestInvertedIndexCalculateAverageDocLenWithNoDocuments(t *testing.T) {
	index := NewInvertedIndex()
	index.CalculateAverageDocLen()
	if index.AverageDocLength != 0 {
		t.Fatalf("expected average length 0, got %f", index.AverageDocLength)
	}
}

func TestIDFScoreAndBM25ScoreEdgeCases(t *testing.T) {
	if idfScore(10, 0) != 0 {
		t.Fatalf("expected idf zero when df is zero, got %f", idfScore(10, 0))
	}

	value := idfScore(10, 2)
	if value <= 0 {
		t.Fatalf("expected positive idf value, got %f", value)
	}

	score := BM25Score(3, value, 100, 80, 1.5, 0.75)
	if score <= 0 {
		t.Fatalf("expected positive bm25 score, got %f", score)
	}

	zeroIDF := BM25Score(3, 0, 100, 80, 1.5, 0.75)
	if zeroIDF != 0 {
		t.Fatalf("expected zero score with idf zero, got %f", zeroIDF)
	}

	if math.IsNaN(score) || math.IsInf(score, 0) {
		t.Fatalf("expected finite score, got %f", score)
	}
}

func TestCountTokenFrequenciesReturnsCountsAndPositions(t *testing.T) {
	tokens := []TokenWithPosition{
		{Token: "go", Position: 0},
		{Token: "idx", Position: 3},
		{Token: "go", Position: 7},
	}

	freq, pos := CountTokenFrequencies(tokens)
	if freq["go"] != 2 || freq["idx"] != 1 {
		t.Fatalf("unexpected frequencies: %#v", freq)
	}
	if len(pos["go"]) != 2 || pos["go"][0] != 0 || pos["go"][1] != 7 {
		t.Fatalf("unexpected positions for go: %#v", pos["go"])
	}
}
