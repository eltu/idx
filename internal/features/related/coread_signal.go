package related

import (
	"fmt"
	"path/filepath"

	"idx/internal/shared/coread"
)

// applyCoReadMatrix adds persistent co-read counts as a ranking signal.
// Scores are normalized by the maximum count for the target file's row.
func applyCoReadMatrix(
	candidates map[string]*candidateScore,
	repo coread.MatrixRepository,
	targetPath, projectRoot, relPath string,
) error {
	counts, err := repo.LoadCoReads(projectRoot, relPath)
	if err != nil {
		return fmt.Errorf("failed to load co-read matrix: %w", err)
	}
	if len(counts) == 0 {
		return nil
	}

	var maxCount uint32
	for _, count := range counts {
		if count > maxCount {
			maxCount = count
		}
	}

	for relCandPath, count := range counts {
		absPath := filepath.Join(projectRoot, filepath.FromSlash(relCandPath))
		if absPath == targetPath {
			continue
		}
		score := float64(count) / float64(maxCount)
		if c, ok := candidates[absPath]; ok {
			c.coReadScore = score
		} else {
			candidates[absPath] = &candidateScore{absPath: absPath, coReadScore: score}
		}
	}
	return nil
}
