package related

import (
	"path/filepath"

	"idx/internal/shared/gitutil"
)

// applyGitCoChange adds git commit co-change scores as a ranking signal.
// Errors are silently ignored so git unavailability degrades gracefully.
func applyGitCoChange(candidates map[string]*candidateScore, projectRoot, relPath string) {
	coChanges, totalCommits, err := gitutil.CoChangedFiles(projectRoot, relPath)
	if err != nil || totalCommits == 0 {
		return
	}
	for relCandPath, count := range coChanges {
		absPath := filepath.Join(projectRoot, filepath.FromSlash(relCandPath))
		score := float64(count) / float64(totalCommits)
		if c, ok := candidates[absPath]; ok {
			c.gitCoChangeScore = score
		} else {
			candidates[absPath] = &candidateScore{absPath: absPath, gitCoChangeScore: score}
		}
	}
}
