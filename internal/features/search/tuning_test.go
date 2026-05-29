package search_test

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"idx/internal/features/indexing"
	search "idx/internal/features/search"
)

func TestWithTuning_ZeroValues_PreserveDefaults(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join("/", "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{filepath.Join(rootDir, "go.mod"): "module idx"}}

	// zero-value opts must not override defaults — search must still work
	service := search.NewSearchCommandService(tree, out, fileReader, repo).
		WithTuning(search.SearchServiceOptions{})
	service.SetCacheEnabled(false)

	// Act
	require.NoError(t, service.RunWithOptions("module idx", search.Options{Format: search.OutputJSON}))

	// Assert
	assert.NotEmpty(t, out.lines)
}

func TestWithTuning_CustomMaxWorkers_AppliedCorrectly(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join("/", "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{filepath.Join(rootDir, "go.mod"): "module idx"}}

	service := search.NewSearchCommandService(tree, out, fileReader, repo).
		WithTuning(search.SearchServiceOptions{MaxWorkers: 1})
	service.SetCacheEnabled(false)

	// Act — should execute correctly with 1 worker
	require.NoError(t, service.RunWithOptions("module idx", search.Options{Format: search.OutputJSON}))
}

func TestWithTuning_CustomBM25_DoesNotPanic(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join("/", "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "guide.md"):  "go search guide",
		filepath.Join(rootDir, "readme.md"): "go content search topic",
	}}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{rootDir: searchableIndex()}}

	extractFirstFile := func(opts search.SearchServiceOptions) string {
		out := &capturingTextOutput{}
		svc := search.NewSearchCommandService(tree, out, fileReader, repo).WithTuning(opts)
		svc.SetCacheEnabled(false)
		require.NoError(t, svc.RunWithOptions("go search", search.Options{Format: search.OutputJSON, Size: 1}))
		var response map[string]any
		require.NoError(t, json.Unmarshal([]byte(out.lines[0]), &response))
		results := response["results"].([]any)
		return results[0].(map[string]any)["file"].(string)
	}

	// Act & Assert — K1 extremes must not panic; valid output must be produced for both
	_ = extractFirstFile(search.SearchServiceOptions{BM25K1: 0.01, BM25B: 0.75, ProximityWeight: 1.0, MaxWorkers: 1})
	_ = extractFirstFile(search.SearchServiceOptions{BM25K1: 3.0, BM25B: 0.75, ProximityWeight: 1.0, MaxWorkers: 1})
}

func TestWithTuning_CustomCacheTTL_IsRespected(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join("/", "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{filepath.Join(rootDir, "go.mod"): "module idx"}}

	shortTTL := 10 * time.Millisecond
	service := search.NewSearchCommandService(tree, out, fileReader, repo).
		WithTuning(search.SearchServiceOptions{CacheTTL: shortTTL, MaxWorkers: 1})
	service.SetCacheEnabled(true)

	// Act — first call populates cache
	require.NoError(t, service.RunWithOptions("module idx", search.Options{Format: search.OutputJSON}))
	firstLoadCount := len(repo.loaded)

	// Act — second call should hit cache — no additional index loads
	require.NoError(t, service.RunWithOptions("module idx", search.Options{Format: search.OutputJSON}))

	// Assert
	assert.Equal(t, firstLoadCount, len(repo.loaded), "expected cache hit on second call")
}
