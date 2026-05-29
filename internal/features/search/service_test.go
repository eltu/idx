package search_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"idx/internal/features/indexing"
	"idx/internal/features/read"
	search "idx/internal/features/search"
)

func TestSearchCommandService_Run_RanksResultsByBM25Score(t *testing.T) {
	t.Parallel()

	// Arrange
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

	// Act
	require.NoError(t, service.Run("go search"))

	// Assert
	require.Len(t, repo.loaded, 1)
	assert.Equal(t, rootDir, repo.loaded[0])
	require.Len(t, out.lines, 8)
	assert.Equal(t, "./guide.md", stripANSICodes(out.lines[1]))
}

func TestSearchCommandService_Run_RequiresAllTermsInDocument(t *testing.T) {
	t.Parallel()

	// Arrange
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

	// Act
	require.NoError(t, service.Run("module idx"))

	// Assert
	assert.Equal(t, "./go.mod", stripANSICodes(out.lines[1]))
}

func TestSearchCommandService_Run_WritesNoResultsMessage(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{rootDir: searchableIndex()}}
	service := newSearchCommandServiceForFunctionalTests(tree, out, fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "guide.md"):  "go search guide",
		filepath.Join(rootDir, "readme.md"): "go content\nsearch topic",
	}}, repo)

	// Act
	require.NoError(t, service.Run("python"))

	// Assert
	assert.Equal(t, "No results found.", out.lines[0])
}

func TestSearchCommandService_Run_ReturnsLoadError(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	repo := &fakeSearchIndexRepository{loadErr: errors.New("boom")}
	service := newSearchCommandServiceForFunctionalTests(tree, &capturingTextOutput{}, fakeSearchFileReader{files: map[string]string{}}, repo)

	// Act
	err := service.Run("go")

	// Assert
	require.Error(t, err)
}

func TestSearchCommandService_Run_ReturnsErrorWhenDependenciesAreNil(t *testing.T) {
	t.Parallel()

	// Arrange
	service := search.NewSearchCommandService(nil, nil, nil, nil)

	// Act & Assert
	require.Error(t, service.Run("module"))
}

func TestSearchCommandService_SetCacheEnabled_WithNilPointerDoesNotPanic(t *testing.T) {
	t.Parallel()

	// Arrange
	var service *search.SearchCommandService

	// Act & Assert — must not panic
	service.SetCacheEnabled(false)
}

func TestSearchCommandService_Run_BoostsDocumentsWithNearbyTerms(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{rootDir: searchableIndexWithProximity()}}
	service := newSearchCommandServiceForFunctionalTests(tree, out, fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "near.txt"): "module idx",
		filepath.Join(rootDir, "far.txt"):  "module\nidx",
	}}, repo)

	// Act
	require.NoError(t, service.Run("module idx"))

	// Assert
	assert.Equal(t, "./near.txt", stripANSICodes(out.lines[1]))
}

func TestSearchCommandService_Run_WritesPathsRelativeToProjectRoot(t *testing.T) {
	t.Parallel()

	// Arrange
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

	// Act
	require.NoError(t, service.Run("module idx"))

	// Assert
	assert.Equal(t, "internal/core/go.mod", stripANSICodes(out.lines[1]))
}

func TestSearchCommandService_Run_SearchesAllProjectIndices(t *testing.T) {
	t.Parallel()

	// Arrange
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

	// Act
	require.NoError(t, service.Run("module idx"))

	// Assert
	assert.Equal(t, "docs/guide.md", stripANSICodes(out.lines[1]))
}

type fakeReadLogRepo struct{}

func (r fakeReadLogRepo) RecordRead(_, _ string) error              { return nil }
func (r fakeReadLogRepo) LoadAll(_ string) ([]read.LogEntry, error) { return nil, nil }

func TestSearchCommandService_WithReadLog_StillWorksCorrectly(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{filepath.Join(rootDir, "go.mod"): "module idx"}}

	service := search.NewSearchCommandService(tree, out, fileReader, repo).
		WithReadLog(fakeReadLogRepo{})
	service.SetCacheEnabled(false)

	// Act
	require.NoError(t, service.Run("module idx"))

	// Assert
	assert.NotEmpty(t, out.lines)
}
