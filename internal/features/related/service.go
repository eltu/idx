package related

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"idx/internal/features/indexing"
	sharedfs "idx/internal/shared/filesystem"
	"idx/internal/shared/output"
	"idx/internal/shared/readlog"
)

type candidateScore struct {
	absPath     string
	termScore   float64
	coReadScore float64
}

func (c *candidateScore) finalScore() float64 {
	return coReadWeight*c.coReadScore + termOverlapWeight*c.termScore
}

func (c *candidateScore) reason() string {
	switch {
	case c.coReadScore > 0 && c.termScore > 0:
		return ReasonBoth
	case c.coReadScore > 0:
		return ReasonCoRead
	default:
		return ReasonTermOverlap
	}
}

// RelatedCommandService finds files related to a target file using co-read affinity
// and BM25 term co-occurrence.
type RelatedCommandService struct {
	projectTree sharedfs.ProjectTree
	indexRepo   indexing.IndexRepository
	logRepo     readlog.LogRepository
	output      output.Writer
}

// NewRelatedCommandService creates the related command use case.
// Example: svc := NewRelatedCommandService(tree, indexRepo, logRepo, out).
func NewRelatedCommandService(
	projectTree sharedfs.ProjectTree,
	indexRepo indexing.IndexRepository,
	logRepo readlog.LogRepository,
	out output.Writer,
) RelatedCommandService {
	return RelatedCommandService{
		projectTree: projectTree,
		indexRepo:   indexRepo,
		logRepo:     logRepo,
		output:      out,
	}
}

// Run finds and writes files related to filePath.
// Example: err := svc.Run("internal/features/search/service.go", opts).
func (svc RelatedCommandService) Run(filePath string, opts Options) error {
	projectRoot, absPath, err := svc.resolveTarget(filePath)
	if err != nil {
		return err
	}

	dirs, err := indexing.IndexedDirectories(svc.projectTree, projectRoot)
	if err != nil {
		return fmt.Errorf("failed to list indexed directories: %w", err)
	}

	logEntries, err := svc.logRepo.LoadAll(projectRoot)
	if err != nil {
		return fmt.Errorf("failed to load read log: %w", err)
	}

	var changedFiles map[string]bool
	if opts.Since != "" {
		changedFiles, err = relatedChangedFiles(projectRoot, opts.Since)
		if err != nil {
			return err
		}
	}

	candidates, err := svc.computeRelated(absPath, projectRoot, dirs, logEntries)
	if err != nil {
		return err
	}

	limit := opts.Size
	if limit <= 0 {
		limit = defaultResultSize
	}

	results := buildResults(candidates, projectRoot, limit+opts.Skip)
	results = applyFilters(results, changedFiles, opts.Ext, projectRoot)
	results = applySkip(results, opts.Skip)
	return writeRelatedResults(results, opts, svc.output)
}

// relatedChangedFiles returns relative paths changed since the given git ref.
func relatedChangedFiles(projectRoot, since string) (map[string]bool, error) {
	cmd := exec.CommandContext(context.Background(), "git", "-C", projectRoot, "diff", "--name-only", since+"...HEAD") // #nosec G204 -- intentional git invocation; ref comes from validated CLI flag
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("invalid git ref %q: %s", since, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("git diff failed for ref %q: %w", since, err)
	}
	files := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			files[line] = true
		}
	}
	return files, nil
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

func (svc RelatedCommandService) resolveTarget(filePath string) (string, string, error) {
	cwd, err := svc.projectTree.CurrentDir()
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve current directory: %w", err)
	}

	absPath := filepath.Clean(filePath)
	if !filepath.IsAbs(absPath) {
		absPath = filepath.Clean(filepath.Join(cwd, filePath))
	}

	projectRoot, err := svc.projectTree.FindGitRoot(cwd)
	if err != nil {
		return "", "", fmt.Errorf("failed to find git root: %w", err)
	}

	return projectRoot, absPath, nil
}

// computeRelated runs the two-phase scoring: collect target terms, then score candidates.
func (svc RelatedCommandService) computeRelated(
	targetPath, projectRoot string,
	dirs []string,
	logEntries []readlog.LogEntry,
) (map[string]*candidateScore, error) {
	targetTerms, err := svc.collectTargetTerms(targetPath, dirs)
	if err != nil {
		return nil, err
	}

	candidates, err := svc.scoreAllCandidates(targetPath, dirs, targetTerms)
	if err != nil {
		return nil, err
	}

	applyCoRead(candidates, logEntries, targetPath, projectRoot)
	return candidates, nil
}

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

// applyCoRead adds temporal proximity scores from the read log.
// Files read within ±2h of the target get a co-read score decaying with distance.
func applyCoRead(
	candidates map[string]*candidateScore,
	logEntries []readlog.LogEntry,
	targetPath, projectRoot string,
) {
	targetReadAt := findTargetReadTime(logEntries, targetPath, projectRoot)
	if targetReadAt.IsZero() {
		return
	}

	for _, entry := range logEntries {
		absPath := filepath.Join(projectRoot, filepath.FromSlash(entry.Path))
		if absPath == targetPath {
			continue
		}
		deltaHours := math.Abs(entry.LastReadAt.Sub(targetReadAt).Hours())
		if deltaHours > coReadWindowHours {
			continue
		}
		score := 1.0 / (1.0 + deltaHours)
		if c := candidates[absPath]; c != nil {
			c.coReadScore = math.Max(c.coReadScore, score)
		} else {
			candidates[absPath] = &candidateScore{absPath: absPath, coReadScore: score}
		}
	}
}

func findTargetReadTime(logEntries []readlog.LogEntry, targetPath, projectRoot string) time.Time {
	for _, entry := range logEntries {
		absPath := filepath.Join(projectRoot, filepath.FromSlash(entry.Path))
		if absPath == targetPath {
			return entry.LastReadAt
		}
	}
	return time.Time{}
}

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
