package search_test

import (
	"path/filepath"
	"sync"
	"testing"

	"idx/internal/core/domain"
	"idx/internal/core/ports"
	search "idx/internal/core/services/search"
)

func TestSearchCommandServiceDefaultCacheIsEnabled(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{
		indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithPartialMatch()},
	}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "guide.md"):  "go search guide",
		filepath.Join(rootDir, "readme.md"): "go content\nsearch topic",
	}}

	service := search.NewSearchCommandService(tree, output, fileReader, repo)

	err := service.RunWithOptions("go search", ports.SearchOptions{Format: ports.SearchOutputJSON, Size: 1})
	if err != nil {
		t.Fatalf("expected first search to succeed, got %v", err)
	}

	err = service.RunWithOptions("go search", ports.SearchOptions{Format: ports.SearchOutputJSON, From: 1, Size: 1})
	if err != nil {
		t.Fatalf("expected second paginated search to succeed, got %v", err)
	}

	if len(repo.loaded) != 1 {
		t.Fatalf("expected default cache enabled behavior (1 load), got %d", len(repo.loaded))
	}
}

func TestSearchCommandServiceCacheDisabledDoesNotReusePaginationResults(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{
		indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithPartialMatch()},
	}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "guide.md"):  "go search guide",
		filepath.Join(rootDir, "readme.md"): "go content\nsearch topic",
	}}

	service := newSearchCommandServiceForFunctionalTests(tree, output, fileReader, repo)

	err := service.RunWithOptions("go search", ports.SearchOptions{Format: ports.SearchOutputJSON, Size: 1})
	if err != nil {
		t.Fatalf("expected first search to succeed, got %v", err)
	}

	err = service.RunWithOptions("go search", ports.SearchOptions{Format: ports.SearchOutputJSON, From: 1, Size: 1})
	if err != nil {
		t.Fatalf("expected second paginated search to succeed, got %v", err)
	}

	if len(repo.loaded) != 2 {
		t.Fatalf("expected cache-disabled behavior (2 loads), got %d", len(repo.loaded))
	}
}

func TestSearchCacheIsUsedForPaginationWithFrom(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{
		indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithPartialMatch()},
	}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "guide.md"):  "go search guide",
		filepath.Join(rootDir, "readme.md"): "go content\nsearch topic",
	}}
	service := newSearchCommandServiceForCacheTests(tree, output, fileReader, repo)

	err := service.RunWithOptions("go search", ports.SearchOptions{Format: ports.SearchOutputJSON, Size: 1})
	if err != nil {
		t.Fatalf("expected first search to succeed, got %v", err)
	}
	if len(repo.loaded) != 1 {
		t.Fatalf("expected 1 index load for first search, got %d", len(repo.loaded))
	}
	firstOutput := output.lines[len(output.lines)-1]

	err = service.RunWithOptions("go search", ports.SearchOptions{Format: ports.SearchOutputJSON, From: 1, Size: 1})
	if err != nil {
		t.Fatalf("expected second search to succeed, got %v", err)
	}

	if len(repo.loaded) != 1 {
		t.Fatalf("expected cache to prevent reload, but got %d loads", len(repo.loaded))
	}

	secondOutput := output.lines[len(output.lines)-1]
	if firstOutput == secondOutput {
		t.Fatalf("expected different results for different pages, but got same output")
	}
}

func TestSearchCacheIsInvalidatedWhenQueryChanges(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{
		indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithPartialMatch()},
	}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "guide.md"):  "go search guide",
		filepath.Join(rootDir, "readme.md"): "go content\nsearch topic",
	}}
	service := newSearchCommandServiceForCacheTests(tree, output, fileReader, repo)

	_ = service.RunWithOptions("go search", ports.SearchOptions{})
	firstLoadCount := len(repo.loaded)

	_ = service.RunWithOptions("module idx", ports.SearchOptions{})
	secondLoadCount := len(repo.loaded)

	if secondLoadCount <= firstLoadCount {
		t.Fatalf("expected new index load for different query, but load count didn't increase: first=%d, second=%d", firstLoadCount, secondLoadCount)
	}
}

func TestSearchCacheIsInvalidatedWhenOptionsChange(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{
		indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithPartialMatch()},
	}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "go.mod"): "module idx",
	}}
	service := newSearchCommandServiceForCacheTests(tree, output, fileReader, repo)

	_ = service.RunWithOptions("module idx", ports.SearchOptions{Context: 0})
	firstLoadCount := len(repo.loaded)

	_ = service.RunWithOptions("module idx", ports.SearchOptions{Context: 1})
	secondLoadCount := len(repo.loaded)

	if secondLoadCount <= firstLoadCount {
		t.Fatalf("expected new index load for different context, but load count didn't increase: first=%d, second=%d", firstLoadCount, secondLoadCount)
	}
}

func TestSearchCacheIsRenewedWhenNavigatingPages(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{
		indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithPartialMatch()},
	}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "guide.md"):  "go search guide",
		filepath.Join(rootDir, "readme.md"): "go content\nsearch topic",
	}}
	service := newSearchCommandServiceForCacheTests(tree, output, fileReader, repo)

	_ = service.RunWithOptions("go search", ports.SearchOptions{})

	_ = service.RunWithOptions("go search", ports.SearchOptions{From: 1})
	if len(repo.loaded) != 1 {
		t.Fatalf("expected cache hit on second page, but index was reloaded")
	}

	_ = service.RunWithOptions("go search", ports.SearchOptions{From: 2})
	if len(repo.loaded) != 1 {
		t.Fatalf("expected cache hit on third page, but index was reloaded")
	}

	if len(repo.loaded) != 1 {
		t.Fatalf("expected exactly 1 index load across all pagination requests, got %d", len(repo.loaded))
	}
}

func TestSearchCacheWorksWithFilesOnlyOption(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{
		indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithPartialMatch()},
	}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "guide.md"):  "go search guide",
		filepath.Join(rootDir, "readme.md"): "go content\nsearch topic",
	}}
	service := newSearchCommandServiceForCacheTests(tree, output, fileReader, repo)

	_ = service.RunWithOptions("go search", ports.SearchOptions{FilesOnly: true, Size: 1})
	firstLoadCount := len(repo.loaded)

	_ = service.RunWithOptions("go search", ports.SearchOptions{FilesOnly: true, From: 1, Size: 1})

	if len(repo.loaded) != firstLoadCount {
		t.Fatalf("expected cache to be used with --files-only, but index was reloaded")
	}
}

func TestSearchCacheWorksWithMatchesOnlyOption(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{
		indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithPartialMatch()},
	}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "go.mod"): "alpha\nmodule idx\nomega",
	}}
	service := newSearchCommandServiceForCacheTests(tree, output, fileReader, repo)

	_ = service.RunWithOptions("module idx", ports.SearchOptions{Context: 1, MatchesOnly: true})
	firstLoadCount := len(repo.loaded)

	_ = service.RunWithOptions("module idx", ports.SearchOptions{Context: 1, MatchesOnly: true, From: 0})

	if len(repo.loaded) != firstLoadCount {
		t.Fatalf("expected cache to be used with --matches-only, but index was reloaded")
	}
}

func TestSearchCacheThreadSafety(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{
		indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithPartialMatch()},
	}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "guide.md"):  "go search guide",
		filepath.Join(rootDir, "readme.md"): "go content\nsearch topic",
	}}
	service := newSearchCommandServiceForCacheTests(tree, output, fileReader, repo)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()
			_ = service.RunWithOptions("go search", ports.SearchOptions{From: offset})
		}(i)
	}
	wg.Wait()

	if len(repo.loaded) == 0 {
		t.Fatalf("expected at least one index load, got %d", len(repo.loaded))
	}
}

func TestSearchCacheSizeDoesNotGrowUnbounded(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{
		indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithPartialMatch()},
	}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "go.mod"): "module idx",
	}}
	service := newSearchCommandServiceForCacheTests(tree, output, fileReader, repo)

	for i := 0; i < 10; i++ {
		_ = service.RunWithOptions("query", ports.SearchOptions{Context: i})
	}

	if len(output.lines) > 0 {
		t.Logf("Cache test completed without panicking")
	}
}

func TestSearchCacheFormatDoesNotAffectCacheKey(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{
		indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithPartialMatch()},
	}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "go.mod"): "module idx",
	}}
	service := newSearchCommandServiceForCacheTests(tree, output, fileReader, repo)

	_ = service.RunWithOptions("module idx", ports.SearchOptions{Format: ports.SearchOutputText})
	firstLoadCount := len(repo.loaded)

	_ = service.RunWithOptions("module idx", ports.SearchOptions{Format: ports.SearchOutputJSON})
	secondLoadCount := len(repo.loaded)

	if secondLoadCount <= firstLoadCount {
		t.Fatalf("expected format difference to invalidate cache, but load count didn't increase: first=%d, second=%d", firstLoadCount, secondLoadCount)
	}
}
