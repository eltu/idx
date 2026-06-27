package related

import (
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"

	"idx/internal/shared/output"
)

func buildResults(candidates map[string]*candidateScore, projectRoot string, limit int) []Result {
	normalizeTermScores(candidates)
	sorted := sortedCandidates(candidates)
	return candidatesToResults(sorted, projectRoot, limit)
}

func normalizeTermScores(candidates map[string]*candidateScore) {
	var maxScore float64
	for _, c := range candidates {
		if c.termScore > maxScore {
			maxScore = c.termScore
		}
	}
	if maxScore == 0 {
		return
	}
	for _, c := range candidates {
		c.termScore /= maxScore
	}
}

func sortedCandidates(candidates map[string]*candidateScore) []*candidateScore {
	list := make([]*candidateScore, 0, len(candidates))
	for _, c := range candidates {
		if c.finalScore() > 0 {
			list = append(list, c)
		}
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].finalScore() > list[j].finalScore()
	})
	return list
}

func candidatesToResults(sorted []*candidateScore, projectRoot string, limit int) []Result {
	if limit > 0 && len(sorted) > limit {
		sorted = sorted[:limit]
	}
	results := make([]Result, 0, len(sorted))
	for _, c := range sorted {
		rel, err := filepath.Rel(projectRoot, c.absPath)
		if err != nil {
			continue
		}
		results = append(results, Result{
			Path:   filepath.ToSlash(rel),
			Score:  math.Round(c.finalScore()*100) / 100,
			Reason: c.reason(),
		})
	}
	return results
}

// applyFilters applies --since and --ext post-filters to the result list.
func applyFilters(results []Result, changedFiles map[string]bool, exts []string, projectRoot string) []Result {
	if changedFiles == nil && len(exts) == 0 {
		return results
	}
	extSet := buildExtSet(exts)
	filtered := make([]Result, 0, len(results))
	for _, r := range results {
		if changedFiles != nil {
			abs := filepath.Join(projectRoot, filepath.FromSlash(r.Path))
			rel, err := filepath.Rel(projectRoot, abs)
			if err != nil || !changedFiles[filepath.ToSlash(rel)] {
				continue
			}
		}
		if len(extSet) > 0 && !extSet[strings.TrimPrefix(filepath.Ext(r.Path), ".")] {
			continue
		}
		filtered = append(filtered, r)
	}
	return filtered
}

func buildExtSet(exts []string) map[string]bool {
	if len(exts) == 0 {
		return nil
	}
	set := make(map[string]bool, len(exts))
	for _, e := range exts {
		set[strings.TrimPrefix(e, ".")] = true
	}
	return set
}

func applySkip(results []Result, skip int) []Result {
	if skip <= 0 {
		return results
	}
	if skip >= len(results) {
		return []Result{}
	}
	return results[skip:]
}

func writeRelatedResults(results []Result, opts Options, out output.Writer) error {
	if opts.Format == OutputJSON {
		return writeRelatedJSON(results, out)
	}
	return writeRelatedText(results, opts, out)
}

func writeRelatedJSON(results []Result, out output.Writer) error {
	encoded, err := json.Marshal(results)
	if err != nil {
		return fmt.Errorf("failed to encode related results as JSON: %w", err)
	}
	return out.WriteLine(string(encoded))
}

const msgNoRelatedFound = "No related files found."

func writeRelatedText(results []Result, opts Options, out output.Writer) error {
	if len(results) == 0 {
		return out.WriteLine(msgNoRelatedFound)
	}
	for _, r := range results {
		var line string
		if opts.Compact {
			line = r.Path
		} else {
			line = fmt.Sprintf("  %-60s %-14s %.2f", r.Path, "("+r.Reason+")", r.Score)
		}
		if err := out.WriteLine(line); err != nil {
			return err
		}
	}
	return nil
}
