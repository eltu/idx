package search_test

import (
	"errors"
	"path/filepath"
	"testing"

	"idx/internal/core/domain"
	search "idx/internal/core/services/search"
)

func TestSearchCommandServiceRunRanksResultsByBM25Score(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "guide.md"):   "go search guide",
		filepath.Join(rootDir, "readme.md"):  "go content\nsearch topic",
		filepath.Join(rootDir, "go.mod"):     "module idx",
		filepath.Join(rootDir, "AGENTS.md"):  "idx",
		filepath.Join(rootDir, ".gitignore"): "module",
	}}
	service := newSearchCommandServiceForFunctionalTests(tree, output, fileReader, repo)

	err := service.Run("go search")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(repo.loaded) != 1 || repo.loaded[0] != rootDir {
		t.Fatalf("expected load for %q, got %v", rootDir, repo.loaded)
	}

	if len(output.lines) != 8 {
		t.Fatalf("expected 8 output lines, got %d: %v", len(output.lines), output.lines)
	}

	if stripANSICodes(output.lines[1]) != "./guide.md (score: 1.0000)" {
		t.Fatalf("expected best result file header first, got %q", output.lines[1])
	}
}

func TestSearchCommandServiceRunRequiresAllTermsInDocument(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "guide.md"):   "go search guide",
		filepath.Join(rootDir, "readme.md"):  "go content\nsearch topic",
		filepath.Join(rootDir, "go.mod"):     "module idx",
		filepath.Join(rootDir, "AGENTS.md"):  "idx",
		filepath.Join(rootDir, ".gitignore"): "module",
	}}
	service := newSearchCommandServiceForFunctionalTests(tree, output, fileReader, repo)

	err := service.Run("module idx")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if stripANSICodes(output.lines[1]) != "./go.mod (score: 1.0000)" {
		t.Fatalf("expected only full match result, got %q", output.lines[1])
	}
}

func TestSearchCommandServiceRunWritesNoResultsMessage(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*domain.InvertedIndex{rootDir: searchableIndex()}}
	service := newSearchCommandServiceForFunctionalTests(tree, output, fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "guide.md"):  "go search guide",
		filepath.Join(rootDir, "readme.md"): "go content\nsearch topic",
	}}, repo)

	err := service.Run("python")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if output.lines[0] != "No results found." {
		t.Fatalf("unexpected output message %q", output.lines[0])
	}
}

func TestSearchCommandServiceRunReturnsLoadError(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	repo := &fakeSearchIndexRepository{loadErr: errors.New("boom")}
	service := newSearchCommandServiceForFunctionalTests(tree, &capturingTextOutput{}, fakeSearchFileReader{files: map[string]string{}}, repo)

	err := service.Run("go")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestSearchCommandServiceRunReturnsErrorWhenDependenciesAreNil(t *testing.T) {
	service := search.NewSearchCommandService(nil, nil, nil, nil)

	err := service.Run("module")
	if err == nil {
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
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithProximity()}}
	service := newSearchCommandServiceForFunctionalTests(tree, output, fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "near.txt"): "module idx",
		filepath.Join(rootDir, "far.txt"):  "module\nidx",
	}}, repo)

	err := service.Run("module idx")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if stripANSICodes(output.lines[1]) != "./near.txt (score: 1.0000)" {
		t.Fatalf("expected nearby terms file first, got %q", output.lines[1])
	}
}

func TestSearchCommandServiceRunWritesPathsRelativeToProjectRoot(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	childDir := filepath.Join(rootDir, "internal", "core")
	tree := searchTreeWithIndexes(rootDir, []string{filepath.Join("internal", "core")})
	tree.currentDir = childDir
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*domain.InvertedIndex{
		rootDir:  searchableIndex(),
		childDir: searchableIndexForRelativePath(),
	}}
	service := newSearchCommandServiceForFunctionalTests(tree, output, fakeSearchFileReader{files: map[string]string{
		filepath.Join(childDir, "go.mod"): "module idx",
	}}, repo)

	err := service.Run("module idx")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if stripANSICodes(output.lines[1]) != "internal/core/go.mod (score: 1.0000)" {
		t.Fatalf("expected project-relative path output, got %q", output.lines[1])
	}
}

func TestSearchCommandServiceRunSearchesAllProjectIndices(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	childDir := filepath.Join(rootDir, "docs")
	tree := searchTreeWithIndexes(rootDir, []string{"docs"})
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*domain.InvertedIndex{
		rootDir:  searchableIndexWithSingleResult("root.md", 1.0, 1.0, []int{1}, []int{2}),
		childDir: searchableIndexWithSingleResult("guide.md", 1.0, 1.0, []int{5}, []int{6}),
	}}
	service := newSearchCommandServiceForFunctionalTests(tree, output, fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "root.md"):   "module idx",
		filepath.Join(childDir, "guide.md"): "module idx",
	}}, repo)

	err := service.Run("module idx")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if stripANSICodes(output.lines[1]) != "docs/guide.md (score: 1.0000)" {
		t.Fatalf("expected child directory file header, got %q", output.lines[1])
	}
}
