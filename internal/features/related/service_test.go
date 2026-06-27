package related

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"idx/internal/features/indexing"
	sharedfs "idx/internal/shared/filesystem"
)

// ---- fakes ----

type fakeProjectTree struct {
	currentDir string
	gitRoot    string
	currentErr error
	gitRootErr error
}

func (f *fakeProjectTree) CurrentDir() (string, error)                         { return f.currentDir, f.currentErr }
func (f *fakeProjectTree) FindGitRoot(_ string) (string, error)                { return f.gitRoot, f.gitRootErr }
func (f *fakeProjectTree) ReadDir(_ string) ([]sharedfs.DirectoryEntry, error) { return nil, nil }
func (f *fakeProjectTree) Exists(_ string) (bool, error)                       { return false, nil }
func (f *fakeProjectTree) RemoveAll(_ string) error                            { return nil }
func (f *fakeProjectTree) WriteFile(_ string, _ []byte) error                  { return nil }

type fakeIndexRepository struct {
	indices map[string]*indexing.InvertedIndex
	loadErr error
}

func (r *fakeIndexRepository) SaveIndex(_ string, _ *indexing.InvertedIndex) error { return nil }
func (r *fakeIndexRepository) LoadIndex(dir string) (*indexing.InvertedIndex, error) {
	if r.loadErr != nil {
		return nil, r.loadErr
	}
	idx, ok := r.indices[dir]
	if !ok {
		return nil, errors.New("index not found")
	}
	return idx, nil
}

type fakeCoReadRepository struct {
	counts    map[string]map[string]uint32
	recordErr error
	loadErr   error
}

func (r *fakeCoReadRepository) RecordCoRead(_, _ string) error { return r.recordErr }
func (r *fakeCoReadRepository) LoadCoReads(_, relPath string) (map[string]uint32, error) {
	if r.loadErr != nil {
		return nil, r.loadErr
	}
	if c, ok := r.counts[relPath]; ok {
		return c, nil
	}
	return map[string]uint32{}, nil
}

type fakeWriter struct {
	lines []string
	err   error
}

func (w *fakeWriter) WriteLine(line string) error {
	if w.err != nil {
		return w.err
	}
	w.lines = append(w.lines, line)
	return nil
}

func (w *fakeWriter) WriteInline(line string) error { return w.WriteLine(line) }

func (w *fakeWriter) joined() string { return strings.Join(w.lines, "\n") }

// ---- helpers ----

const (
	testRoot      = "/project"
	testDir       = "/project/internal/search"
	targetFile    = "service.go"
	candidateA    = "query.go"
	targetAbsPath = testDir + "/" + targetFile
)

func buildIndex(avgLen float64, docs map[string]int, terms map[string]*indexing.TermStats) *indexing.InvertedIndex {
	idx := indexing.NewInvertedIndex()
	idx.AverageDocLength = avgLen
	idx.DocumentCount = len(docs)
	for name, length := range docs {
		idx.Documents[name] = &indexing.DocStats{Name: name, Length: length}
	}
	idx.Terms = terms
	return idx
}

// ---- candidateScore ----

func TestCandidateScore_FinalScore_WeightsAllSignals(t *testing.T) {
	t.Parallel()

	c := &candidateScore{termScore: 1.0, coReadScore: 1.0, gitCoChangeScore: 1.0}
	expected := gitCoChangeWeight*1.0 + coReadMatrixWeight*1.0 + termOverlapWeight*1.0

	assert.InDelta(t, expected, c.finalScore(), 1e-9)
}

func TestCandidateScore_Reason_BothSignals(t *testing.T) {
	t.Parallel()

	c := &candidateScore{termScore: 0.5, coReadScore: 0.5}
	assert.Equal(t, ReasonBoth, c.reason())
}

func TestCandidateScore_Reason_GitOnly(t *testing.T) {
	t.Parallel()

	c := &candidateScore{gitCoChangeScore: 0.5}
	assert.Equal(t, ReasonGit, c.reason())
}

func TestCandidateScore_Reason_CoReadOnly(t *testing.T) {
	t.Parallel()

	c := &candidateScore{coReadScore: 0.5}
	assert.Equal(t, ReasonCoRead, c.reason())
}

func TestCandidateScore_Reason_TermOverlapOnly(t *testing.T) {
	t.Parallel()

	c := &candidateScore{termScore: 0.5}
	assert.Equal(t, ReasonTermOverlap, c.reason())
}

func TestCandidateScore_Reason_AllThreeSignals_ReturnsBoth(t *testing.T) {
	t.Parallel()

	c := &candidateScore{gitCoChangeScore: 0.5, coReadScore: 0.5, termScore: 0.5}
	assert.Equal(t, ReasonBoth, c.reason())
}

// ---- extractTargetTerms ----

func TestExtractTargetTerms_TargetPresent_AccumulatesTF(t *testing.T) {
	t.Parallel()

	idx := buildIndex(5, map[string]int{targetFile: 10, candidateA: 5}, map[string]*indexing.TermStats{
		"search": {IDF: 1.0, Docs: map[string]*indexing.DocTermStats{
			targetFile: {TF: 3},
			candidateA: {TF: 1},
		}},
		"query": {IDF: 1.0, Docs: map[string]*indexing.DocTermStats{
			targetFile: {TF: 2},
		}},
	})

	out := make(map[string]int)
	extractTargetTerms(idx, testDir, targetAbsPath, out)

	assert.Equal(t, 3, out["search"])
	assert.Equal(t, 2, out["query"])
	assert.NotContains(t, out, candidateA)
}

func TestExtractTargetTerms_TargetAbsent_EmptyMap(t *testing.T) {
	t.Parallel()

	idx := buildIndex(5, map[string]int{candidateA: 5}, map[string]*indexing.TermStats{})

	out := make(map[string]int)
	extractTargetTerms(idx, testDir, targetAbsPath, out)

	assert.Empty(t, out)
}

// ---- scoreDirectory ----

func TestScoreDirectory_ExcludesTarget_OnlyScoresCandidates(t *testing.T) {
	t.Parallel()

	idx := buildIndex(5, map[string]int{targetFile: 10, candidateA: 5}, map[string]*indexing.TermStats{
		"search": {IDF: 1.0, Docs: map[string]*indexing.DocTermStats{
			targetFile: {TF: 3},
			candidateA: {TF: 2},
		}},
	})

	candidates := make(map[string]*candidateScore)
	targetTerms := map[string]int{"search": 3}
	scoreDirectory(candidates, idx, testDir, targetAbsPath, targetTerms)

	assert.NotContains(t, candidates, targetAbsPath)
	assert.Contains(t, candidates, testDir+"/"+candidateA)
	assert.Positive(t, candidates[testDir+"/"+candidateA].termScore)
}

func TestScoreDirectory_ZeroAvgDocLen_SkipsIndex(t *testing.T) {
	t.Parallel()

	idx := buildIndex(0, map[string]int{candidateA: 5}, map[string]*indexing.TermStats{
		"term": {IDF: 1.0, Docs: map[string]*indexing.DocTermStats{candidateA: {TF: 1}}},
	})

	candidates := make(map[string]*candidateScore)
	scoreDirectory(candidates, idx, testDir, targetAbsPath, map[string]int{"term": 1})

	assert.Empty(t, candidates)
}

func TestScoreDirectory_NoMatchingTerms_SkipsCandidate(t *testing.T) {
	t.Parallel()

	idx := buildIndex(5, map[string]int{candidateA: 5}, map[string]*indexing.TermStats{
		"irrelevant": {IDF: 1.0, Docs: map[string]*indexing.DocTermStats{candidateA: {TF: 1}}},
	})

	candidates := make(map[string]*candidateScore)
	scoreDirectory(candidates, idx, testDir, targetAbsPath, map[string]int{"search": 3})

	assert.Empty(t, candidates)
}

// ---- applyCoReadMatrix ----

func TestApplyCoReadMatrix_WithCounts_AddsCoReadScore(t *testing.T) {
	t.Parallel()

	relCandPath := "internal/search/" + candidateA
	repo := &fakeCoReadRepository{
		counts: map[string]map[string]uint32{
			"internal/search/" + targetFile: {relCandPath: 5},
		},
	}

	candidates := make(map[string]*candidateScore)
	err := applyCoReadMatrix(candidates, repo, testDir+"/"+targetFile, testRoot, "internal/search/"+targetFile)

	require.NoError(t, err)
	absA := filepath.Join(testRoot, relCandPath)
	require.Contains(t, candidates, absA)
	assert.InDelta(t, 1.0, candidates[absA].coReadScore, 1e-9) // single entry = max = 1.0
}

func TestApplyCoReadMatrix_NormalizesScores(t *testing.T) {
	t.Parallel()

	relA := "a.go"
	relB := "b.go"
	relTarget := "target.go"
	repo := &fakeCoReadRepository{
		counts: map[string]map[string]uint32{
			relTarget: {relA: 10, relB: 5},
		},
	}

	candidates := make(map[string]*candidateScore)
	err := applyCoReadMatrix(candidates, repo, filepath.Join(testRoot, relTarget), testRoot, relTarget)

	require.NoError(t, err)
	assert.InDelta(t, 1.0, candidates[filepath.Join(testRoot, relA)].coReadScore, 1e-9)
	assert.InDelta(t, 0.5, candidates[filepath.Join(testRoot, relB)].coReadScore, 1e-9)
}

func TestApplyCoReadMatrix_EmptyCounts_NoChange(t *testing.T) {
	t.Parallel()

	repo := &fakeCoReadRepository{counts: map[string]map[string]uint32{}}
	candidates := make(map[string]*candidateScore)

	err := applyCoReadMatrix(candidates, repo, testDir+"/"+targetFile, testRoot, "internal/search/"+targetFile)

	require.NoError(t, err)
	assert.Empty(t, candidates)
}

func TestApplyCoReadMatrix_LoadError_ReturnsError(t *testing.T) {
	t.Parallel()

	repo := &fakeCoReadRepository{loadErr: errors.New("disk error")}
	candidates := make(map[string]*candidateScore)

	err := applyCoReadMatrix(candidates, repo, testDir+"/"+targetFile, testRoot, "internal/search/"+targetFile)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "co-read matrix")
}

func TestApplyCoReadMatrix_ExcludesTarget(t *testing.T) {
	t.Parallel()

	relTarget := "internal/search/" + targetFile
	repo := &fakeCoReadRepository{
		counts: map[string]map[string]uint32{
			relTarget: {relTarget: 10, "internal/search/" + candidateA: 5},
		},
	}

	candidates := make(map[string]*candidateScore)
	err := applyCoReadMatrix(candidates, repo, testDir+"/"+targetFile, testRoot, relTarget)

	require.NoError(t, err)
	assert.NotContains(t, candidates, testDir+"/"+targetFile)
}

// ---- applyGitCoChange ----

func TestApplyGitCoChange_InRealRepo_AddsScores(t *testing.T) {
	t.Parallel()

	// Arrange: real git repo with two commits that co-change target and candidate.
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)
	writeGitFile(t, tmpDir, "target.go", "package main")
	writeGitFile(t, tmpDir, "sibling.go", "package main")
	runGit(t, tmpDir, "add", ".")
	runGit(t, tmpDir, "commit", "-m", "first commit")

	candidates := make(map[string]*candidateScore)
	applyGitCoChange(candidates, tmpDir, "target.go")

	// "sibling.go" appeared in the same commit as "target.go".
	absA := filepath.Join(tmpDir, "sibling.go")
	require.Contains(t, candidates, absA)
	assert.InDelta(t, 1.0, candidates[absA].gitCoChangeScore, 1e-9)
}

func TestApplyGitCoChange_FileWithNoHistory_NoChange(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)
	writeGitFile(t, tmpDir, "untracked.go", "package main")

	candidates := make(map[string]*candidateScore)
	applyGitCoChange(candidates, tmpDir, "untracked.go")

	assert.Empty(t, candidates)
}

func TestApplyGitCoChange_GitNotAvailable_NoopGracefully(t *testing.T) {
	t.Parallel()

	// Not a git repo — git will fail.
	tmpDir := t.TempDir()
	candidates := make(map[string]*candidateScore)
	applyGitCoChange(candidates, tmpDir, "file.go")

	assert.Empty(t, candidates)
}

// ---- normalizeTermScores ----

func TestNormalizeTermScores_NormalizesMax(t *testing.T) {
	t.Parallel()

	candidates := map[string]*candidateScore{
		"a": {termScore: 4.0},
		"b": {termScore: 2.0},
	}
	normalizeTermScores(candidates)

	assert.InDelta(t, 1.0, candidates["a"].termScore, 1e-9)
	assert.InDelta(t, 0.5, candidates["b"].termScore, 1e-9)
}

func TestNormalizeTermScores_AllZero_NoChange(t *testing.T) {
	t.Parallel()

	candidates := map[string]*candidateScore{"a": {termScore: 0}}
	normalizeTermScores(candidates)
	assert.Equal(t, 0.0, candidates["a"].termScore)
}

// ---- sortedCandidates ----

func TestSortedCandidates_SortsByFinalScoreDescending(t *testing.T) {
	t.Parallel()

	candidates := map[string]*candidateScore{
		"a": {absPath: "a", termScore: 0.2},
		"b": {absPath: "b", termScore: 1.0},
		"c": {absPath: "c", termScore: 0.5},
	}
	sorted := sortedCandidates(candidates)

	require.Len(t, sorted, 3)
	assert.Equal(t, "b", sorted[0].absPath)
	assert.Equal(t, "c", sorted[1].absPath)
	assert.Equal(t, "a", sorted[2].absPath)
}

func TestSortedCandidates_ZeroScore_Excluded(t *testing.T) {
	t.Parallel()

	candidates := map[string]*candidateScore{
		"a": {absPath: "a", termScore: 0, coReadScore: 0, gitCoChangeScore: 0},
		"b": {absPath: "b", termScore: 0.5},
	}
	sorted := sortedCandidates(candidates)

	require.Len(t, sorted, 1)
	assert.Equal(t, "b", sorted[0].absPath)
}

// ---- candidatesToResults ----

func TestCandidatesToResults_LimitApplied(t *testing.T) {
	t.Parallel()

	absA := filepath.Join(testRoot, "a.go")
	absB := filepath.Join(testRoot, "b.go")
	sorted := []*candidateScore{
		{absPath: absA, termScore: 1.0},
		{absPath: absB, termScore: 0.5},
	}
	results := candidatesToResults(sorted, testRoot, 1)

	require.Len(t, results, 1)
	assert.Equal(t, "a.go", results[0].Path)
}

func TestCandidatesToResults_RelativePath_UsesSlash(t *testing.T) {
	t.Parallel()

	absA := filepath.Join(testRoot, "internal", "a.go")
	sorted := []*candidateScore{{absPath: absA, termScore: 1.0}}
	results := candidatesToResults(sorted, testRoot, 10)

	require.Len(t, results, 1)
	assert.Equal(t, "internal/a.go", results[0].Path)
}

func TestCandidatesToResults_ScoreRoundedToTwoDecimals(t *testing.T) {
	t.Parallel()

	absA := filepath.Join(testRoot, "a.go")
	sorted := []*candidateScore{{absPath: absA, termScore: 1.0 / 3.0}}
	results := candidatesToResults(sorted, testRoot, 10)

	expected := math.Round((termOverlapWeight*1.0/3.0)*100) / 100
	assert.InDelta(t, expected, results[0].Score, 1e-9)
}

// ---- writeRelatedText ----

func TestWriteRelatedText_NoResults_PrintsNoRelatedMessage(t *testing.T) {
	t.Parallel()

	w := &fakeWriter{}
	err := writeRelatedText(nil, Options{}, w)

	require.NoError(t, err)
	assert.Contains(t, w.joined(), "No related files found.")
}

func TestWriteRelatedText_WithResults_FormatsLines(t *testing.T) {
	t.Parallel()

	results := []Result{
		{Path: "internal/search/query.go", Score: 0.92, Reason: ReasonBoth},
	}
	w := &fakeWriter{}
	err := writeRelatedText(results, Options{}, w)

	require.NoError(t, err)
	require.Len(t, w.lines, 1)
	assert.Contains(t, w.lines[0], "internal/search/query.go")
	assert.Contains(t, w.lines[0], "(both)")
	assert.Contains(t, w.lines[0], "0.92")
}

func TestWriteRelatedText_Compact_OutputsPathOnly(t *testing.T) {
	t.Parallel()

	results := []Result{{Path: "a.go", Score: 0.8, Reason: ReasonCoRead}}
	w := &fakeWriter{}
	err := writeRelatedText(results, Options{Compact: true}, w)

	require.NoError(t, err)
	assert.Equal(t, "a.go", w.lines[0])
}

func TestWriteRelatedText_WriterError_ReturnsError(t *testing.T) {
	t.Parallel()

	results := []Result{{Path: "a.go", Score: 0.5, Reason: ReasonTermOverlap}}
	w := &fakeWriter{err: errors.New("write failed")}
	err := writeRelatedText(results, Options{}, w)

	require.Error(t, err)
}

// ---- writeRelatedJSON ----

func TestWriteRelatedJSON_EmptySlice_WritesEmptyArray(t *testing.T) {
	t.Parallel()

	w := &fakeWriter{}
	err := writeRelatedJSON(nil, w)

	require.NoError(t, err)
	require.Len(t, w.lines, 1)
	assert.JSONEq(t, "null", w.lines[0])
}

func TestWriteRelatedJSON_WithResults_WritesValidJSON(t *testing.T) {
	t.Parallel()

	results := []Result{
		{Path: "a.go", Score: 0.75, Reason: ReasonCoRead},
	}
	w := &fakeWriter{}
	require.NoError(t, writeRelatedJSON(results, w))

	var parsed []Result
	require.NoError(t, json.Unmarshal([]byte(w.lines[0]), &parsed))
	require.Len(t, parsed, 1)
	assert.Equal(t, "a.go", parsed[0].Path)
	assert.InDelta(t, 0.75, parsed[0].Score, 1e-9)
}

// ---- RelatedCommandService.Run ----

func newTestService(
	currentDir, gitRoot string,
	indices map[string]*indexing.InvertedIndex,
	coReadCounts map[string]map[string]uint32,
	w *fakeWriter,
) RelatedCommandService {
	tree := &fakeProjectTree{currentDir: currentDir, gitRoot: gitRoot}
	indexRepo := &fakeIndexRepository{indices: indices}
	coReadRepo := &fakeCoReadRepository{counts: coReadCounts}
	return NewRelatedCommandService(tree, indexRepo, coReadRepo, w)
}

func TestRelatedCommandService_Run_NoIndexedDirs_ReturnsNoResultsMessage(t *testing.T) {
	t.Parallel()

	// Arrange
	w := &fakeWriter{}
	svc := newTestService(testRoot, testRoot, map[string]*indexing.InvertedIndex{}, nil, w)

	// Act
	err := svc.Run("service.go", Options{Format: OutputText, Size: 10})

	// Assert
	require.NoError(t, err)
	assert.Contains(t, w.joined(), msgNoRelatedFound)
}

func TestRelatedCommandService_Run_CurrentDirError_ReturnsError(t *testing.T) {
	t.Parallel()

	// Arrange
	w := &fakeWriter{}
	tree := &fakeProjectTree{currentErr: errors.New("cwd failed")}
	svc := NewRelatedCommandService(tree, &fakeIndexRepository{}, &fakeCoReadRepository{}, w)

	// Act
	err := svc.Run("service.go", Options{})

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "current directory")
}

func TestRelatedCommandService_Run_GitRootError_ReturnsError(t *testing.T) {
	t.Parallel()

	// Arrange
	w := &fakeWriter{}
	tree := &fakeProjectTree{
		currentDir: testRoot,
		gitRootErr: errors.New("not a git repo"),
	}
	svc := NewRelatedCommandService(tree, &fakeIndexRepository{}, &fakeCoReadRepository{}, w)

	// Act
	err := svc.Run("service.go", Options{})

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git root")
}

func TestRelatedCommandService_Run_CoReadRepoError_ReturnsError(t *testing.T) {
	t.Parallel()

	// Arrange
	w := &fakeWriter{}
	tree := &fakeProjectTree{currentDir: testRoot, gitRoot: testRoot}
	coReadRepo := &fakeCoReadRepository{loadErr: errors.New("matrix read failed")}
	svc := NewRelatedCommandService(tree, &fakeIndexRepository{indices: map[string]*indexing.InvertedIndex{}}, coReadRepo, w)

	// Act
	err := svc.Run("service.go", Options{})

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "co-read matrix")
}

func TestRelatedCommandService_Run_JSONFormat_WritesJSON(t *testing.T) {
	t.Parallel()

	// Arrange
	w := &fakeWriter{}
	svc := newTestService(testRoot, testRoot, map[string]*indexing.InvertedIndex{}, nil, w)

	// Act
	err := svc.Run("service.go", Options{Format: OutputJSON, Size: 10})

	// Assert
	require.NoError(t, err)
	assert.Len(t, w.lines, 1)
}

// ---- Run with --since filter ----

func TestRelatedCommandService_Run_InvalidSince_ReturnsError(t *testing.T) {
	t.Parallel()

	// Arrange
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)
	writeGitFile(t, tmpDir, "service.go", "package main")
	runGit(t, tmpDir, "add", ".")
	runGit(t, tmpDir, "commit", "-m", "init")

	w := &fakeWriter{}
	tree := &fakeProjectTree{currentDir: tmpDir, gitRoot: tmpDir}
	svc := NewRelatedCommandService(tree, &fakeIndexRepository{indices: map[string]*indexing.InvertedIndex{}}, &fakeCoReadRepository{}, w)

	// Act
	err := svc.Run("service.go", Options{Since: "bad-ref-xyz"})

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad-ref-xyz")
}

// ---- collectTargetTerms error path ----

func TestCollectTargetTerms_LoadIndexError_ReturnsError(t *testing.T) {
	t.Parallel()

	// Arrange
	tree := &fakeProjectTree{currentDir: testRoot, gitRoot: testRoot}
	indexRepo := &fakeIndexRepository{loadErr: errors.New("disk full")}
	svc := NewRelatedCommandService(tree, indexRepo, &fakeCoReadRepository{}, &fakeWriter{})

	// Act
	_, err := svc.collectTargetTerms(targetAbsPath, []string{testDir})

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "index")
}

// ---- scoreAllCandidates error path ----

func TestScoreAllCandidates_LoadIndexError_ReturnsError(t *testing.T) {
	t.Parallel()

	// Arrange
	tree := &fakeProjectTree{currentDir: testRoot, gitRoot: testRoot}
	indexRepo := &fakeIndexRepository{loadErr: errors.New("io failure")}
	svc := NewRelatedCommandService(tree, indexRepo, &fakeCoReadRepository{}, &fakeWriter{})

	// Act
	_, err := svc.scoreAllCandidates(targetAbsPath, []string{testDir}, map[string]int{"search": 2})

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "index")
}

// ---- writeRelatedJSON writer error ----

func TestWriteRelatedJSON_WriterError_ReturnsError(t *testing.T) {
	t.Parallel()

	results := []Result{{Path: "a.go", Score: 0.5, Reason: ReasonTermOverlap}}
	w := &fakeWriter{err: errors.New("write failed")}
	err := writeRelatedJSON(results, w)

	require.Error(t, err)
}

// ---- buildResults ----

func TestBuildResults_LimitZero_UsesAllResults(t *testing.T) {
	t.Parallel()

	absA := filepath.Join(testRoot, "a.go")
	absB := filepath.Join(testRoot, "b.go")
	candidates := map[string]*candidateScore{
		absA: {absPath: absA, termScore: 1.0},
		absB: {absPath: absB, termScore: 0.5},
	}
	results := buildResults(candidates, testRoot, 0)

	assert.Len(t, results, 2)
}

func TestBuildResults_EmptyCandidates_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	results := buildResults(map[string]*candidateScore{}, testRoot, 10)
	assert.Empty(t, results)
}

// ---- applyFilters ----

func TestApplyFilters_NoFilters_ReturnsUnchanged(t *testing.T) {
	t.Parallel()

	results := []Result{{Path: "a.go"}, {Path: "b.go"}}
	got := applyFilters(results, nil, nil, testRoot)
	assert.Len(t, got, 2)
}

func TestApplyFilters_ExtFilter_KeepsMatchingExtensions(t *testing.T) {
	t.Parallel()

	results := []Result{
		{Path: "internal/a.go"},
		{Path: "internal/b.md"},
		{Path: "internal/c.go"},
	}
	got := applyFilters(results, nil, []string{"go"}, testRoot)

	require.Len(t, got, 2)
	assert.Equal(t, "internal/a.go", got[0].Path)
	assert.Equal(t, "internal/c.go", got[1].Path)
}

func TestApplyFilters_ExtFilterWithDot_StripsLeadingDot(t *testing.T) {
	t.Parallel()

	results := []Result{{Path: "a.go"}, {Path: "b.ts"}}
	got := applyFilters(results, nil, []string{".go"}, testRoot)

	require.Len(t, got, 1)
	assert.Equal(t, "a.go", got[0].Path)
}

func TestApplyFilters_ChangedFilesFilter_KeepsOnlyChanged(t *testing.T) {
	t.Parallel()

	changedFiles := map[string]bool{"internal/a.go": true}
	results := []Result{
		{Path: "internal/a.go"},
		{Path: "internal/b.go"},
	}
	got := applyFilters(results, changedFiles, nil, testRoot)

	require.Len(t, got, 1)
	assert.Equal(t, "internal/a.go", got[0].Path)
}

func TestApplyFilters_BothFilters_Intersection(t *testing.T) {
	t.Parallel()

	changedFiles := map[string]bool{"internal/a.go": true, "internal/b.md": true}
	results := []Result{
		{Path: "internal/a.go"},
		{Path: "internal/b.md"},
		{Path: "internal/c.go"},
	}
	got := applyFilters(results, changedFiles, []string{"go"}, testRoot)

	require.Len(t, got, 1)
	assert.Equal(t, "internal/a.go", got[0].Path)
}

// ---- applySkip ----

func TestApplySkip_ZeroSkip_ReturnsAll(t *testing.T) {
	t.Parallel()

	results := []Result{{Path: "a.go"}, {Path: "b.go"}}
	assert.Len(t, applySkip(results, 0), 2)
}

func TestApplySkip_PositiveSkip_RemovesFirst(t *testing.T) {
	t.Parallel()

	results := []Result{{Path: "a.go"}, {Path: "b.go"}, {Path: "c.go"}}
	got := applySkip(results, 1)

	require.Len(t, got, 2)
	assert.Equal(t, "b.go", got[0].Path)
}

func TestApplySkip_SkipBeyondLen_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	results := []Result{{Path: "a.go"}}
	assert.Empty(t, applySkip(results, 5))
}

// ---- git helpers shared with test style ----

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test.com")
	runGit(t, dir, "config", "user.name", "Test")
}

func writeGitFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("writeGitFile: %v", err)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...) //nolint:gosec
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
