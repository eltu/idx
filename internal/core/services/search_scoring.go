package services

import (
	"math"
	"path/filepath"

	"idx/internal/core/domain"
)

func sortResults(results []searchResult) {
	for i := 1; i < len(results); i++ {
		for j := i; j > 0; j-- {
			left, right := results[j-1], results[j]
			if left.score > right.score {
				break
			}
			if left.score == right.score {
				leftPath := filepath.Join(left.directoryPath, left.fileName)
				rightPath := filepath.Join(right.directoryPath, right.fileName)
				if leftPath <= rightPath {
					break
				}
			}
			results[j-1], results[j] = results[j], results[j-1]
		}
	}
}

// normalizeScores scales all scores in-place to [0, 1] using min-max.
// Each directory is an independent BM25 corpus: small directories produce
// higher IDF than large ones, making raw scores incomparable across directories.
func normalizeScores(scores map[string]float64) {
	if len(scores) == 0 {
		return
	}
	minScore, maxScore := scoreRange(scores)
	spread := maxScore - minScore
	if spread == 0 {
		for key := range scores {
			scores[key] = 1.0
		}
		return
	}
	for key, score := range scores {
		scores[key] = (score - minScore) / spread
	}
}

func scoreRange(scores map[string]float64) (float64, float64) {
	minScore := math.MaxFloat64
	maxScore := -math.MaxFloat64
	for _, score := range scores {
		if score < minScore {
			minScore = score
		}
		if score > maxScore {
			maxScore = score
		}
	}
	return minScore, maxScore
}

func scoreDocuments(index *domain.InvertedIndex, terms []string) map[string]float64 {
	matchingDocuments := documentsContainingAllTerms(index, terms)
	if len(matchingDocuments) == 0 {
		return map[string]float64{}
	}

	scores := make(map[string]float64)
	for _, term := range terms {
		termStats := index.Terms[term]
		if termStats == nil {
			continue
		}

		addTermScores(scores, index, termStats, matchingDocuments)
	}

	applyProximityBonus(scores, index, terms, matchingDocuments)

	return scores
}

func applyProximityBonus(scores map[string]float64, index *domain.InvertedIndex, terms []string, matchingDocuments map[string]struct{}) {
	for filePath := range matchingDocuments {
		scores[filePath] += proximityBonusForDocument(index, filePath, terms)
	}
}

func proximityBonusForDocument(index *domain.InvertedIndex, filePath string, terms []string) float64 {
	if len(terms) < 2 {
		return 0
	}

	totalPairScore := 0.0
	pairCount := 0
	for termIndex := 0; termIndex < len(terms)-1; termIndex++ {
		distance, ok := minimumDistanceForTermPair(index, filePath, terms[termIndex], terms[termIndex+1])
		if !ok {
			continue
		}

		totalPairScore += 1.0 / (1.0 + float64(distance))
		pairCount++
	}

	if pairCount == 0 {
		return 0
	}

	return proximityWeight * (totalPairScore / float64(pairCount))
}

func minimumDistanceForTermPair(index *domain.InvertedIndex, filePath string, leftTerm string, rightTerm string) (int, bool) {
	leftDocTerm := index.Terms[leftTerm].Docs[filePath]
	rightDocTerm := index.Terms[rightTerm].Docs[filePath]
	if len(leftDocTerm.Positions) == 0 || len(rightDocTerm.Positions) == 0 {
		return 0, false
	}

	minDistance := math.MaxInt
	for _, leftPos := range leftDocTerm.Positions {
		for _, rightPos := range rightDocTerm.Positions {
			distance := absInt(leftPos - rightPos)
			if distance < minDistance {
				minDistance = distance
			}
		}
	}

	return minDistance, true
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}

	return value
}

func documentsContainingAllTerms(index *domain.InvertedIndex, terms []string) map[string]struct{} {
	if len(terms) == 0 {
		return map[string]struct{}{}
	}

	firstTermStats := index.Terms[terms[0]]
	if firstTermStats == nil {
		return map[string]struct{}{}
	}

	matchingDocuments := make(map[string]struct{})
	for filePath := range firstTermStats.Docs {
		matchingDocuments[filePath] = struct{}{}
	}

	for _, term := range terms[1:] {
		termStats := index.Terms[term]
		if termStats == nil {
			return map[string]struct{}{}
		}

		filterDocumentsByTerm(matchingDocuments, termStats)
	}

	return matchingDocuments
}

func filterDocumentsByTerm(matchingDocuments map[string]struct{}, termStats *domain.TermStats) {
	for filePath := range matchingDocuments {
		if _, exists := termStats.Docs[filePath]; exists {
			continue
		}

		delete(matchingDocuments, filePath)
	}
}

func addTermScores(scores map[string]float64, index *domain.InvertedIndex, termStats *domain.TermStats, matchingDocuments map[string]struct{}) {
	for filePath, docTerm := range termStats.Docs {
		if _, exists := matchingDocuments[filePath]; !exists {
			continue
		}

		docStats := index.Documents[filePath]
		if docStats == nil {
			continue
		}

		scores[filePath] += domain.BM25Score(docTerm.TF, termStats.IDF, docStats.Length, index.AverageDocLength, bm25K1, bm25B)
	}
}
