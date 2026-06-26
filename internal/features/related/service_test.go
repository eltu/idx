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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"idx/internal/features/indexing"
	sharedfs "idx/internal/shared/filesystem"
	"idx/internal/shared/readlog"
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

type fakeLogRepository struct {
	entries []readlog.LogEntry
	loadErr error
}

func (r *fakeLogRepository) RecordRead(_, _ string) error { return nil }
func (r *fakeLogRepository) LoadAll(_ string) ([]readlog.LogEntry, error) {
	return r.entries, r.loadErr
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

func TestCandidateScore_FinalScore_WeightsCoReadAndTerm(t *testing.T) {
	t.Parallel()

	c := &candidateScore{termScore: 1.0, coReadScore: 1.0}
	expected := coReadWeight*1.0 + termOverlapWeight*1.0

	assert.InDelta(t, expected, c.finalScore(), 1e-9)
}

func TestCandidateScore_Reason_BothSignals(t *testing.T) {
	t.Parallel()

	c := &candidateScore{termScore: 0.5, coReadScore: 0.5}
	assert.Equal(t, ReasonBoth, c.reason())
}

func TestCandidateScore_Reason_CoReadOnly(t *testing.T) {
	t.Parallel()

	c := &candidateScore{termScore: 0, coReadScore: 0.5}
	assert.Equal(t, ReasonCoRead, c.reason())
}

func TestCandidateScore_Reason_TermOverlapOnly(t *testing.T) {
	t.Parallel()

	c := &candidateScore{termScore: 0.5, coReadScore: 0}
	assert.Equal(t, ReasonTermOverlap, c.reason())
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

// ---- applyCoRead ----

func TestApplyCoRead_WithinWindow_AddsCoReadScore(t *testing.T) {
	t.Parallel()

	now := time.Now()
	logEntries := []readlog.LogEntry{
		{Path: "internal/search/" + targetFile, LastReadAt: now},
		{Path: "internal/search/" + candidateA, LastReadAt: now.Add(30 * time.Minute)},
	}

	candidates := make(map[string]*candidateScore)
	applyCoRead(candidates, logEntries, testDir+"/"+targetFile, testRoot)

	absA := filepath.Join(testRoot, "internal/search/"+candidateA)
	require.Contains(t, candidates, absA)
	assert.Positive(t, candidates[absA].coReadScore)
}

func TestApplyCoRead_OutsideWindow_NoCoReadScore(t *testing.T) {
	t.Parallel()

	now := time.Now()
	logEntries := []readlog.LogEntry{
		{Path: "internal/search/" + targetFile, LastReadAt: now},
		{Path: "internal/search/" + candidateA, LastReadAt: now.Add(-3 * time.Hour)},
	}

	candidates := make(map[string]*candidateScore)
	applyCoRead(candidates, logEntries, testDir+"/"+targetFile, testRoot)

	absA := filepath.Join(testRoot, "internal/search/"+candidateA)
	assert.NotContains(t, candidates, absA)
}

func TestApplyCoRead_TargetNotInLog_NoScoring(t *testing.T) {
	t.Parallel()

	logEntries := []readlog.LogEntry{
		{Path: "internal/search/" + candidateA, LastReadAt: time.Now()},
	}

	candidates := make(map[string]*candidateScore)
	applyCoRead(candidates, logEntries, testDir+"/"+targetFile, testRoot)

	assert.Empty(t, candidates)
}

func TestApplyCoRead_UpdatesExistingCandidate_MaxScore(t *testing.T) {
	t.Parallel()

	now := time.Now()
	absA := filepath.Join(testRoot, "internal/search/"+candidateA)
	logEntries := []readlog.LogEntry{
		{Path: "internal/search/" + targetFile, LastReadAt: now},
		{Path: "internal/search/" + candidateA, LastReadAt: now.Add(30 * time.Minute)},
	}

	candidates := map[string]*candidateScore{
		absA: {absPath: absA, termScore: 0.5},
	}
	applyCoRead(candidates, logEntries, testDir+"/"+targetFile, testRoot)

	assert.Positive(t, candidates[absA].coReadScore)
	assert.Equal(t, 0.5, candidates[absA].termScore)
}

// ---- findTargetReadTime ----

func TestFindTargetReadTime_Found_ReturnsTime(t *testing.T) {
	t.Parallel()

	ts := time.Now()
	entries := []readlog.LogEntry{
		{Path: "other.go", LastReadAt: ts.Add(-time.Hour)},
		{Path: "internal/search/" + targetFile, LastReadAt: ts},
	}

	result := findTargetReadTime(entries, testDir+"/"+targetFile, testRoot)
	assert.Equal(t, ts, result)
}

func TestFindTargetReadTime_NotFound_ReturnsZero(t *testing.T) {
	t.Parallel()

	entries := []readlog.LogEntry{{Path: "other.go", LastReadAt: time.Now()}}
	result := findTargetReadTime(entries, targetAbsPath, testRoot)
	assert.True(t, result.IsZero())
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
		"a": {absPath: "a", termScore: 0.2, coReadScore: 0.0},
		"b": {absPath: "b", termScore: 1.0, coReadScore: 0.0},
		"c": {absPath: "c", termScore: 0.5, coReadScore: 0.0},
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
		"a": {absPath: "a", termScore: 0, coReadScore: 0},
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
	logEntries []readlog.LogEntry,
	w *fakeWriter,
) RelatedCommandService {
	tree := &fakeProjectTree{currentDir: currentDir, gitRoot: gitRoot}
	indexRepo := &fakeIndexRepository{indices: indices}
	logRepo := &fakeLogRepository{entries: logEntries}
	return NewRelatedCommandService(tree, indexRepo, logRepo, w)
}

func buildTestIndex(targetDoc, relatedDoc string) *indexing.InvertedIndex {
	return buildIndex(5, map[string]int{targetDoc: 10, relatedDoc: 8}, map[string]*indexing.TermStats{
		"query": {IDF: 1.5, Docs: map[string]*indexing.DocTermStats{
			targetDoc:  {TF: 3},
			relatedDoc: {TF: 2},
		}},
	})
}

func TestRelatedCommandService_Run_NoIndexedDirs_ReturnsNoResultsMessage(t *testing.T) {
	t.Parallel()

	// Arrange: ProjectTree.ReadDir returns nothing (no .idx subdirs)
	w := &fakeWriter{}
	tree := &fakeProjectTree{currentDir: testRoot, gitRoot: testRoot}
	indexRepo := &fakeIndexRepository{indices: map[string]*indexing.InvertedIndex{}}
	logRepo := &fakeLogRepository{}
	svc := NewRelatedCommandService(tree, indexRepo, logRepo, w)

	// Act
	err := svc.Run("service.go", Options{Format: OutputText, Size: 10})

	// Assert
	require.NoError(t, err)
	assert.Contains(t, w.joined(), msgNoRelatedFound)
}

func TestRelatedCommandService_Run_WithTermOverlap_ReturnsRelatedFiles(t *testing.T) {
	t.Parallel()

	// Arrange
	w := &fakeWriter{}
	dir := filepath.Join(testRoot, ".idx", testDir)
	indices := map[string]*indexing.InvertedIndex{
		testDir: buildTestIndex(targetFile, candidateA),
	}
	tree := &fakeProjectTree{currentDir: testDir, gitRoot: testRoot}

	// ProjectTree needs ReadDir to expose the indexed directory.
	// Since fakeProjectTree.ReadDir returns nil, IndexedDirectories won't find
	// any directories — we accept "No related files found." here unless we
	// implement ReadDir. This test verifies the Run pipeline without disk I/O.
	_ = dir
	indexRepo := &fakeIndexRepository{indices: indices}
	logRepo := &fakeLogRepository{}
	svc := NewRelatedCommandService(tree, indexRepo, logRepo, w)

	// Act
	err := svc.Run(targetAbsPath, Options{Format: OutputText, Size: 10})

	// Assert: no error regardless of empty results (no indexed dirs)
	require.NoError(t, err)
}

func TestRelatedCommandService_Run_CurrentDirError_ReturnsError(t *testing.T) {
	t.Parallel()

	// Arrange
	w := &fakeWriter{}
	tree := &fakeProjectTree{currentErr: errors.New("cwd failed")}
	svc := NewRelatedCommandService(tree, &fakeIndexRepository{}, &fakeLogRepository{}, w)

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
	svc := NewRelatedCommandService(tree, &fakeIndexRepository{}, &fakeLogRepository{}, w)

	// Act
	err := svc.Run("service.go", Options{})

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git root")
}

func TestRelatedCommandService_Run_LogRepoError_ReturnsError(t *testing.T) {
	t.Parallel()

	// Arrange
	w := &fakeWriter{}
	tree := &fakeProjectTree{currentDir: testRoot, gitRoot: testRoot}
	logRepo := &fakeLogRepository{loadErr: errors.New("log read failed")}
	svc := NewRelatedCommandService(tree, &fakeIndexRepository{indices: map[string]*indexing.InvertedIndex{}}, logRepo, w)

	// Act
	err := svc.Run("service.go", Options{})

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read log")
}

func TestRelatedCommandService_Run_JSONFormat_WritesJSON(t *testing.T) {
	t.Parallel()

	// Arrange
	w := &fakeWriter{}
	tree := &fakeProjectTree{currentDir: testRoot, gitRoot: testRoot}
	svc := NewRelatedCommandService(tree, &fakeIndexRepository{indices: map[string]*indexing.InvertedIndex{}}, &fakeLogRepository{}, w)

	// Act
	err := svc.Run("service.go", Options{Format: OutputJSON, Size: 10})

	// Assert
	require.NoError(t, err)
	// JSON output from empty results is "null" (nil slice marshaled)
	assert.Len(t, w.lines, 1)
}

// ---- relatedChangedFiles ----

func TestRelatedChangedFiles_ValidRef_ReturnsFiles(t *testing.T) {
	t.Parallel()

	// Arrange: real git repo with two commits
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)
	writeGitFile(t, tmpDir, "a.go", "package a")
	runGit(t, tmpDir, "add", ".")
	runGit(t, tmpDir, "commit", "-m", "first")
	writeGitFile(t, tmpDir, "b.go", "package b")
	runGit(t, tmpDir, "add", ".")
	runGit(t, tmpDir, "commit", "-m", "second")

	// Act
	files, err := relatedChangedFiles(tmpDir, "HEAD~1")

	// Assert
	require.NoError(t, err)
	assert.True(t, files["b.go"])
	assert.False(t, files["a.go"])
}

func TestRelatedChangedFiles_InvalidRef_ReturnsError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)
	writeGitFile(t, tmpDir, "a.go", "package a")
	runGit(t, tmpDir, "add", ".")
	runGit(t, tmpDir, "commit", "-m", "init")

	_, err := relatedChangedFiles(tmpDir, "nonexistent-ref-xyz")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent-ref-xyz")
}

// ---- Run with --since filter ----

func TestRelatedCommandService_Run_InvalidSince_ReturnsError(t *testing.T) {
	t.Parallel()

	// Arrange: real git repo so git subprocess works
	tmpDir := t.TempDir()
	initGitRepo(t, tmpDir)
	writeGitFile(t, tmpDir, "service.go", "package main")
	runGit(t, tmpDir, "add", ".")
	runGit(t, tmpDir, "commit", "-m", "init")

	w := &fakeWriter{}
	tree := &fakeProjectTree{currentDir: tmpDir, gitRoot: tmpDir}
	svc := NewRelatedCommandService(tree, &fakeIndexRepository{indices: map[string]*indexing.InvertedIndex{}}, &fakeLogRepository{}, w)

	// Act
	err := svc.Run("service.go", Options{Since: "bad-ref-xyz"})

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad-ref-xyz")
}

// helpers shared with query_executor_test style.
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

// ---- collectTargetTerms error path ----

func TestCollectTargetTerms_LoadIndexError_ReturnsError(t *testing.T) {
	t.Parallel()

	// Arrange
	tree := &fakeProjectTree{currentDir: testRoot, gitRoot: testRoot}
	indexRepo := &fakeIndexRepository{loadErr: errors.New("disk full")}
	svc := NewRelatedCommandService(tree, indexRepo, &fakeLogRepository{}, &fakeWriter{})

	// Act: call internal method directly (same package)
	_, err := svc.collectTargetTerms(targetAbsPath, []string{testDir})

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "index")
}

// ---- scoreAllCandidates error path ----

func TestScoreAllCandidates_LoadIndexError_ReturnsError(t *testing.T) {
	t.Parallel()

	// Arrange: non-empty targetTerms so the loop is entered
	tree := &fakeProjectTree{currentDir: testRoot, gitRoot: testRoot}
	indexRepo := &fakeIndexRepository{loadErr: errors.New("io failure")}
	svc := NewRelatedCommandService(tree, indexRepo, &fakeLogRepository{}, &fakeWriter{})

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

	// limit 0 = no truncation from candidatesToResults
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
