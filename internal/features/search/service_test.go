package search_test

import (
	"errors"
	"path/filepath"
	"testing"

	"idx/internal/features/indexing"
	"idx/internal/features/read"
	search "idx/internal/features/search"
)

func TestSearchCommandServiceRunRanksResultsByBM25Score(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "guide.md"):   "go search guide",
		filepath.Join(rootDir, "readme.md"):  "go content\nsearch topic",
		filepath.Join(rootDir, "go.mod"):     "module idx",
		filepath.Join(rootDir, "AGENTS.md"):  "idx",
		filepath.Join(rootDir, ".gitignore"): "module",
	}}
	service := newSearchCommandServiceForFunctionalTests(tree, out, fileReader, repo)

	if err := service.Run("go search"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(repo.loaded) != 1 || repo.loaded[0] != rootDir {
		t.Fatalf("expected load for %q, got %v", rootDir, repo.loaded)
	}
	if len(out.lines) != 8 {
		t.Fatalf("expected 8 output lines, got %d: %v", len(out.lines), out.lines)
	}
	if stripANSICodes(out.lines[1]) != "./guide.md" {
		t.Fatalf("expected best result file header first, got %q", out.lines[1])
	}
}

func TestSearchCommandServiceRunRequiresAllTermsInDocument(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "guide.md"):   "go search guide",
		filepath.Join(rootDir, "readme.md"):  "go content\nsearch topic",
		filepath.Join(rootDir, "go.mod"):     "module idx",
		filepath.Join(rootDir, "AGENTS.md"):  "idx",
		filepath.Join(rootDir, ".gitignore"): "module",
	}}
	service := newSearchCommandServiceForFunctionalTests(tree, out, fileReader, repo)

	if err := service.Run("module idx"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if stripANSICodes(out.lines[1]) != "./go.mod" {
		t.Fatalf("expected only full match result, got %q", out.lines[1])
	}
}

func TestSearchCommandServiceRunWritesNoResultsMessage(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{rootDir: searchableIndex()}}
	service := newSearchCommandServiceForFunctionalTests(tree, out, fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "guide.md"):  "go search guide",
		filepath.Join(rootDir, "readme.md"): "go content\nsearch topic",
	}}, repo)

	if err := service.Run("python"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if out.lines[0] != "No results found." {
		t.Fatalf("unexpected output message %q", out.lines[0])
	}
}

func TestSearchCommandServiceRunReturnsLoadError(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	repo := &fakeSearchIndexRepository{loadErr: errors.New("boom")}
	service := newSearchCommandServiceForFunctionalTests(tree, &capturingTextOutput{}, fakeSearchFileReader{files: map[string]string{}}, repo)

	if err := service.Run("go"); err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestSearchCommandServiceRunReturnsErrorWhenDependenciesAreNil(t *testing.T) {
	service := search.NewSearchCommandService(nil, nil, nil, nil)

	if err := service.Run("module"); err == nil {
		t.Fatal("expected dependency validation error, got nil")
	}
}

func TestSearchCommandServiceSetCacheEnabledWithNilPointerDoesNotPanic(t *testing.T) {
	var service *search.SearchCommandService
	service.SetCacheEnabled(false)
}

func TestSearchCommandServiceRunBoostsDocumentsWithNearbyTerms(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{rootDir: searchableIndexWithProximity()}}
	service := newSearchCommandServiceForFunctionalTests(tree, out, fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "near.txt"): "module idx",
		filepath.Join(rootDir, "far.txt"):  "module\nidx",
	}}, repo)

	if err := service.Run("module idx"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if stripANSICodes(out.lines[1]) != "./near.txt" {
		t.Fatalf("expected nearby terms file first, got %q", out.lines[1])
	}
}

func TestSearchCommandServiceRunWritesPathsRelativeToProjectRoot(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	childDir := filepath.Join(rootDir, "internal", "core")
	tree := searchTreeWithIndexes(rootDir, []string{filepath.Join("internal", "core")})
	tree.currentDir = childDir
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{
		rootDir:  searchableIndex(),
		childDir: searchableIndexForRelativePath(),
	}}
	service := newSearchCommandServiceForFunctionalTests(tree, out, fakeSearchFileReader{files: map[string]string{
		filepath.Join(childDir, "go.mod"): "module idx",
	}}, repo)

	if err := service.Run("module idx"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if stripANSICodes(out.lines[1]) != "internal/core/go.mod" {
		t.Fatalf("expected project-relative path output, got %q", out.lines[1])
	}
}

func TestSearchCommandServiceRunSearchesAllProjectIndices(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	childDir := filepath.Join(rootDir, "docs")
	tree := searchTreeWithIndexes(rootDir, []string{"docs"})
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{
		rootDir:  searchableIndexWithSingleResult("root.md", 1.0, 1.0, []int{1}, []int{2}),
		childDir: searchableIndexWithSingleResult("guide.md", 1.0, 1.0, []int{5}, []int{6}),
	}}
	service := newSearchCommandServiceForFunctionalTests(tree, out, fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "root.md"):   "module idx",
		filepath.Join(childDir, "guide.md"): "module idx",
	}}, repo)

	if err := service.Run("module idx"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if stripANSICodes(out.lines[1]) != "docs/guide.md" {
		t.Fatalf("expected child directory file header, got %q", out.lines[1])
	}
}

func TestSearchCommandServiceRunWithExplainShowsScoreInTextOutput(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "go.mod"): "module idx",
	}}
	service := newSearchCommandServiceForFunctionalTests(tree, out, fileReader, repo)

	if err := service.RunWithOptions("module idx", search.Options{Explain: true}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if stripANSICodes(out.lines[1]) != "./go.mod (score: 1.0000)" {
		t.Fatalf("expected score when explain is enabled, got %q", out.lines[1])
	}
}

type fakeReadLogRepo struct{}

func (r fakeReadLogRepo) RecordRead(_, _ string) error              { return nil }
func (r fakeReadLogRepo) LoadAll(_ string) ([]read.LogEntry, error) { return nil, nil }

func TestSearchCommandServiceWithReadLogSearchStillWorks(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{filepath.Join(rootDir, "go.mod"): "module idx"}}

	service := search.NewSearchCommandService(tree, out, fileReader, repo).
		WithReadLog(fakeReadLogRepo{})
	service.SetCacheEnabled(false)

	if err := service.Run("module idx"); err != nil {
		t.Fatalf("expected no error with read log wired, got %v", err)
	}
	if len(out.lines) == 0 {
		t.Fatal("expected output lines")
	}
}
