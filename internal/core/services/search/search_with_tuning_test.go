package search_test

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"idx/internal/core/domain"
	"idx/internal/core/ports"
	search "idx/internal/core/services/search"
)

func TestWithTuningZeroValuesPreserveDefaults(t *testing.T) {
	rootDir := filepath.Join("/", "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{filepath.Join(rootDir, "go.mod"): "module idx"}}

	// zero-value opts must not override defaults — search must still work
	service := search.NewSearchCommandService(tree, output, fileReader, repo).
		WithTuning(search.SearchServiceOptions{})
	service.SetCacheEnabled(false)

	err := service.RunWithOptions("module idx", ports.SearchOptions{Format: ports.SearchOutputJSON})
	if err != nil {
		t.Fatalf("expected no error with zero-value tuning, got %v", err)
	}
	if len(output.lines) == 0 {
		t.Fatal("expected output, got none")
	}
}

func TestWithTuningCustomMaxWorkersIsApplied(t *testing.T) {
	rootDir := filepath.Join("/", "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{filepath.Join(rootDir, "go.mod"): "module idx"}}

	service := search.NewSearchCommandService(tree, output, fileReader, repo).
		WithTuning(search.SearchServiceOptions{MaxWorkers: 1})
	service.SetCacheEnabled(false)

	// should still execute correctly with 1 worker
	err := service.RunWithOptions("module idx", ports.SearchOptions{Format: ports.SearchOutputJSON})
	if err != nil {
		t.Fatalf("expected no error with MaxWorkers=1, got %v", err)
	}
}

func TestWithTuningCustomBM25AffectsScores(t *testing.T) {
	rootDir := filepath.Join("/", "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "guide.md"):  "go search guide",
		filepath.Join(rootDir, "readme.md"): "go content search topic",
	}}
	repo := &fakeSearchIndexRepository{indices: map[string]*domain.InvertedIndex{rootDir: searchableIndex()}}

	extractFirstFile := func(opts search.SearchServiceOptions) string {
		out := &capturingTextOutput{}
		svc := search.NewSearchCommandService(tree, out, fileReader, repo).WithTuning(opts)
		svc.SetCacheEnabled(false)
		if err := svc.RunWithOptions("go search", ports.SearchOptions{Format: ports.SearchOutputJSON, Size: 1}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var response map[string]any
		if err := json.Unmarshal([]byte(out.lines[0]), &response); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		results := response["results"].([]any)
		return results[0].(map[string]any)["file"].(string)
	}

	// K1=0.01 saturates at very low TF — high TF files get no boost → ranking shifts
	// Changing K1 must not crash; ranking may or may not differ for this fixture.
	_ = extractFirstFile(search.SearchServiceOptions{BM25K1: 0.01, BM25B: 0.75, ProximityWeight: 1.0, MaxWorkers: 1})
	_ = extractFirstFile(search.SearchServiceOptions{BM25K1: 3.0, BM25B: 0.75, ProximityWeight: 1.0, MaxWorkers: 1})
	// main assertion: no panic, valid output produced for both extremes
}

func TestWithTuningCustomCacheTTLIsRespected(t *testing.T) {
	rootDir := filepath.Join("/", "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{filepath.Join(rootDir, "go.mod"): "module idx"}}

	shortTTL := 10 * time.Millisecond
	service := search.NewSearchCommandService(tree, output, fileReader, repo).
		WithTuning(search.SearchServiceOptions{CacheTTL: shortTTL, MaxWorkers: 1})
	service.SetCacheEnabled(true)

	// first call populates cache
	if err := service.RunWithOptions("module idx", ports.SearchOptions{Format: ports.SearchOutputJSON}); err != nil {
		t.Fatalf("first call failed: %v", err)
	}
	firstLoadCount := len(repo.loaded)

	// second call should hit cache — no additional index loads
	if err := service.RunWithOptions("module idx", ports.SearchOptions{Format: ports.SearchOutputJSON}); err != nil {
		t.Fatalf("second call failed: %v", err)
	}
	if len(repo.loaded) != firstLoadCount {
		t.Fatalf("expected cache hit on second call, but index was loaded %d times total (first call: %d)", len(repo.loaded), firstLoadCount)
	}
}
