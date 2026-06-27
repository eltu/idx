package related

import (
	"fmt"
	"path/filepath"

	"idx/internal/features/indexing"
)

func (svc RelatedCommandService) collectTargetTerms(targetPath string, dirs []string) (map[string]int, error) {
	terms := make(map[string]int)
	for _, dir := range dirs {
		index, err := svc.indexRepo.LoadIndex(dir)
		if err != nil {
			return nil, fmt.Errorf("failed to load index for %q: %w", dir, err)
		}
		extractTargetTerms(index, dir, targetPath, terms)
	}
	return terms, nil
}

// extractTargetTerms accumulates TF values for the target file's terms from one index.
func extractTargetTerms(index *indexing.InvertedIndex, dir, targetPath string, out map[string]int) {
	for docName := range index.Documents {
		if filepath.Join(dir, docName) != targetPath {
			continue
		}
		for term, stats := range index.Terms {
			if dtStats, ok := stats.Docs[docName]; ok {
				out[term] += dtStats.TF
			}
		}
	}
}

func (svc RelatedCommandService) scoreAllCandidates(
	targetPath string,
	dirs []string,
	targetTerms map[string]int,
) (map[string]*candidateScore, error) {
	candidates := make(map[string]*candidateScore)
	if len(targetTerms) == 0 {
		return candidates, nil
	}

	for _, dir := range dirs {
		index, err := svc.indexRepo.LoadIndex(dir)
		if err != nil {
			return nil, fmt.Errorf("failed to load index for %q: %w", dir, err)
		}
		scoreDirectory(candidates, index, dir, targetPath, targetTerms)
	}

	return candidates, nil
}

// scoreDirectory scores each doc in one index using targetTerms as the BM25 query.
func scoreDirectory(
	candidates map[string]*candidateScore,
	index *indexing.InvertedIndex,
	dir, targetPath string,
	targetTerms map[string]int,
) {
	if index.AverageDocLength == 0 {
		return
	}

	for docName, doc := range index.Documents {
		absPath := filepath.Join(dir, docName)
		if absPath == targetPath {
			continue
		}
		score := computeTermScore(index, doc, docName, targetTerms)
		if score <= 0 {
			continue
		}

		if c := candidates[absPath]; c != nil {
			c.termScore += score
		} else {
			candidates[absPath] = &candidateScore{absPath: absPath, termScore: score}
		}
	}
}

func computeTermScore(
	index *indexing.InvertedIndex,
	doc *indexing.DocStats,
	docName string,
	targetTerms map[string]int,
) float64 {
	var score float64
	for term := range targetTerms {
		termStats, ok := index.Terms[term]
		if !ok {
			continue
		}
		dtStats, ok := termStats.Docs[docName]
		if !ok {
			continue
		}
		score += indexing.BM25Score(dtStats.TF, termStats.IDF, doc.Length, index.AverageDocLength, bm25K1, bm25B)
	}
	return score
}
