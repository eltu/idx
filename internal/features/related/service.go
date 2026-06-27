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

	"idx/internal/features/indexing"
	"idx/internal/shared/coread"
	sharedfs "idx/internal/shared/filesystem"
	"idx/internal/shared/output"
)

type candidateScore struct {
	absPath          string
	termScore        float64
	coReadScore      float64
	gitCoChangeScore float64
}

func (c *candidateScore) finalScore() float64 {
	return gitCoChangeWeight*c.gitCoChangeScore +
		coReadMatrixWeight*c.coReadScore +
		termOverlapWeight*c.termScore
}

func (c *candidateScore) reason() string {
	active := boolToInt(c.gitCoChangeScore > 0) + boolToInt(c.coReadScore > 0) + boolToInt(c.termScore > 0)
	if active > 1 {
		return ReasonBoth
	}
	return dominantReason(c)
}

func dominantReason(c *candidateScore) string {
	switch {
	case c.gitCoChangeScore > 0:
		return ReasonGit
	case c.coReadScore > 0:
		return ReasonCoRead
	default:
		return ReasonTermOverlap
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// RelatedCommandService finds files related to a target file using git co-change
// history, persistent co-read affinity, and BM25 term co-occurrence.
type RelatedCommandService struct {
	projectTree sharedfs.ProjectTree
	indexRepo   indexing.IndexRepository
	coReadRepo  coread.MatrixRepository
	output      output.Writer
}

// NewRelatedCommandService creates the related command use case.
// Example: svc := NewRelatedCommandService(tree, indexRepo, coReadRepo, out).
func NewRelatedCommandService(
	projectTree sharedfs.ProjectTree,
	indexRepo indexing.IndexRepository,
	coReadRepo coread.MatrixRepository,
	out output.Writer,
) RelatedCommandService {
	return RelatedCommandService{
		projectTree: projectTree,
		indexRepo:   indexRepo,
		coReadRepo:  coReadRepo,
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

	changedFiles, err := resolveChangedFiles(opts.Since, projectRoot)
	if err != nil {
		return err
	}

	candidates, err := svc.computeRelated(absPath, projectRoot, dirs)
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

func resolveChangedFiles(since, projectRoot string) (map[string]bool, error) {
	if since == "" {
		return nil, nil
	}
	return relatedChangedFiles(projectRoot, since)
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

// computeRelated runs term scoring, co-read matrix, and git co-change signals.
func (svc RelatedCommandService) computeRelated(
	targetPath, projectRoot string,
	dirs []string,
) (map[string]*candidateScore, error) {
	relPath, err := filepath.Rel(projectRoot, targetPath)
	if err != nil {
		return nil, fmt.Errorf("failed to compute relative path for %q: %w", targetPath, err)
	}
	relPath = filepath.ToSlash(relPath)

	targetTerms, err := svc.collectTargetTerms(targetPath, dirs)
	if err != nil {
		return nil, err
	}

	candidates, err := svc.scoreAllCandidates(targetPath, dirs, targetTerms)
	if err != nil {
		return nil, err
	}

	if err := applyCoReadMatrix(candidates, svc.coReadRepo, targetPath, projectRoot, relPath); err != nil {
		return nil, err
	}
	applyGitCoChange(candidates, projectRoot, relPath)

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

// applyGitCoChange adds git commit co-change scores as a ranking signal.
// Errors are silently ignored so git unavailability degrades gracefully.
func applyGitCoChange(candidates map[string]*candidateScore, projectRoot, relPath string) {
	coChanges, totalCommits, err := gitCoChangedFiles(projectRoot, relPath)
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

// gitCoChangedFiles returns a map of relative paths → commit count alongside the
// target file, plus the total number of commits that touched relPath.
// Uses two git calls: one to fetch commit SHAs, one to fetch all files from those commits.
func gitCoChangedFiles(projectRoot, relPath string) (map[string]int, int, error) {
	shas, err := gitCommitSHAs(projectRoot, relPath)
	if err != nil || len(shas) == 0 {
		return map[string]int{}, 0, err
	}

	raw, err := gitCommitFiles(projectRoot, shas)
	if err != nil {
		return map[string]int{}, 0, err
	}

	return parseCoChangeFiles(raw, relPath, len(shas))
}

// gitCommitSHAs returns the full SHA hashes of commits that touched relPath.
func gitCommitSHAs(projectRoot, relPath string) ([]string, error) {
	cmd := exec.CommandContext( // #nosec G204 -- intentional git invocation; relPath is a sanitized relative path
		context.Background(), "git", "-C", projectRoot, "log", "--format=%H", "--", relPath,
	)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("git log failed for %q: %s", relPath, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("git log failed for %q: %w", relPath, err)
	}
	return strings.Fields(string(out)), nil
}

// gitCommitFiles returns all file names touched by the given commits.
// git diff-tree outputs each SHA as a header line, then the file list.
func gitCommitFiles(projectRoot string, shas []string) (string, error) {
	// --root includes root commits (no parent) in the diff output.
	args := append([]string{"-C", projectRoot, "diff-tree", "--root", "-r", "--name-only"}, shas...) //nolint:gocritic
	cmd := exec.CommandContext(context.Background(), "git", args...)                                 // #nosec G204
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff-tree failed: %w", err)
	}
	return string(out), nil
}

// parseCoChangeFiles counts how often each file co-appears with relPath across
// totalCommits commits. SHA header lines (40 hex chars) and relPath itself are skipped.
func parseCoChangeFiles(raw, relPath string, totalCommits int) (map[string]int, int, error) {
	coChanges := make(map[string]int)
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || isGitSHA(line) || line == relPath {
			continue
		}
		coChanges[line]++
	}
	return coChanges, totalCommits, nil
}

// isGitSHA returns true if s is a 40-character lowercase hexadecimal string (SHA-1).
func isGitSHA(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
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
