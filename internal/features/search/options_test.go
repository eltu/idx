package search_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"idx/internal/features/indexing"
	search "idx/internal/features/search"
)

func TestSearchCommandService_RunWithOptions_ReturnsJSONOutput(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{filepath.Join(rootDir, "go.mod"): "module idx"}}
	service := newSearchCommandServiceForFunctionalTests(tree, out, fileReader, repo)

	// Act
	require.NoError(t, service.RunWithOptions("module idx", search.Options{Format: search.OutputJSON}))

	// Assert
	var response map[string]any
	require.NoError(t, json.Unmarshal([]byte(out.lines[0]), &response))
	assert.Equal(t, float64(1), response["count"])
}

func TestSearchCommandService_RunWithOptions_ReturnsPrettyJSONOutput(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{filepath.Join(rootDir, "go.mod"): "module idx"}}
	service := newSearchCommandServiceForFunctionalTests(tree, out, fileReader, repo)

	// Act
	require.NoError(t, service.RunWithOptions("module idx", search.Options{Format: search.OutputJSON, PrettyJSON: true}))

	// Assert
	assert.Contains(t, out.lines[0], "\n")
}

func TestSearchCommandService_RunWithOptions_IncludesContextLines(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{filepath.Join(rootDir, "go.mod"): "alpha\nmodule idx\nomega"}}
	service := newSearchCommandServiceForFunctionalTests(tree, out, fileReader, repo)

	// Act
	require.NoError(t, service.RunWithOptions("module idx", search.Options{Context: 1}))

	// Assert
	assert.Equal(t, "├── 1: alpha", stripANSICodes(out.lines[2]))
}

func TestSearchCommandService_RunWithOptions_SizeRestrictsResultCount(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "guide.md"):  "go search guide",
		filepath.Join(rootDir, "readme.md"): "go content\nsearch topic",
	}}
	service := newSearchCommandServiceForFunctionalTests(tree, out, fileReader, repo)

	// Act
	require.NoError(t, service.RunWithOptions("go search", search.Options{Format: search.OutputJSON, Size: 1}))

	// Assert
	var response map[string]any
	require.NoError(t, json.Unmarshal([]byte(out.lines[0]), &response))
	results := response["results"].([]any)
	assert.Len(t, results, 1)
}

func TestSearchCommandService_RunWithOptions_FromAndSizePaginateResults(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "guide.md"):  "go search guide",
		filepath.Join(rootDir, "readme.md"): "go content\nsearch topic",
	}}
	service := newSearchCommandServiceForFunctionalTests(tree, out, fileReader, repo)

	// Act
	require.NoError(t, service.RunWithOptions("go search", search.Options{Format: search.OutputJSON, From: 1, Size: 1}))

	// Assert
	var response map[string]any
	require.NoError(t, json.Unmarshal([]byte(out.lines[0]), &response))
	payload := response["results"].([]any)[0].(map[string]any)
	assert.Equal(t, "./readme.md", payload["file"])
}

func TestSearchCommandService_RunWithOptions_DisplaysMatchCountInTextFormat(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "guide.md"):  "go search guide",
		filepath.Join(rootDir, "readme.md"): "go content\nsearch topic",
	}}
	service := newSearchCommandServiceForFunctionalTests(tree, out, fileReader, repo)

	// Act
	require.NoError(t, service.RunWithOptions("go search", search.Options{Format: search.OutputText}))

	// Assert
	assert.Contains(t, out.lines[0], "Found 2 file(s)")
}

func TestSearchCommandService_RunWithOptions_DisplaysMatchCountWithPagination(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "guide.md"):  "go search guide",
		filepath.Join(rootDir, "readme.md"): "go content\nsearch topic",
	}}
	service := newSearchCommandServiceForFunctionalTests(tree, out, fileReader, repo)

	// Act
	require.NoError(t, service.RunWithOptions("go search", search.Options{Format: search.OutputText, Size: 1}))

	// Assert
	assert.Contains(t, out.lines[0], "showing 1")
}

func TestSearchCommandService_RunWithOptions_AgentCompact_OutputsTokenEfficientText(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{filepath.Join(rootDir, "go.mod"): "   module idx   "}}
	service := newSearchCommandServiceForFunctionalTests(tree, out, fileReader, repo)

	// Act
	require.NoError(t, service.RunWithOptions("module idx", search.Options{Format: search.OutputText, AgentCompact: true}))

	// Assert
	require.Len(t, out.lines, 2)
	assert.Equal(t, "./go.mod", out.lines[0])
	assert.Equal(t, "1:module idx", out.lines[1])
}

func TestSearchCommandService_RunWithOptions_FilesOnly_ReturnsPathsOnlyInText(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "guide.md"):  "go search guide",
		filepath.Join(rootDir, "readme.md"): "go content\nsearch topic",
	}}
	service := newSearchCommandServiceForFunctionalTests(tree, out, fileReader, repo)

	// Act
	require.NoError(t, service.RunWithOptions("go search", search.Options{Format: search.OutputText, FilesOnly: true}))

	// Assert
	assert.Len(t, out.lines, 3)
}

func TestSearchCommandService_RunWithOptions_FilesOnly_ReturnsJSONArray(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "guide.md"):  "go search guide",
		filepath.Join(rootDir, "readme.md"): "go content\nsearch topic",
	}}
	service := newSearchCommandServiceForFunctionalTests(tree, out, fileReader, repo)

	// Act
	require.NoError(t, service.RunWithOptions("go search", search.Options{Format: search.OutputJSON, FilesOnly: true}))

	// Assert
	var payload []string
	require.NoError(t, json.Unmarshal([]byte(out.lines[0]), &payload))
	assert.Len(t, payload, 2)
}

func TestSearchCommandService_RunWithOptions_FilesOnlyWithPrettyJSON(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "guide.md"):  "go search guide",
		filepath.Join(rootDir, "readme.md"): "go content\nsearch topic",
	}}
	service := newSearchCommandServiceForFunctionalTests(tree, out, fileReader, repo)

	// Act
	require.NoError(t, service.RunWithOptions("go search", search.Options{Format: search.OutputJSON, FilesOnly: true, PrettyJSON: true}))

	// Assert
	assert.Contains(t, out.lines[0], "\n")
}

func TestSearchCommandService_RunWithOptions_MetadataOnlyPathFilter(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	childDir := filepath.Join(rootDir, "internal", "core")
	tree := searchTreeWithIndexes(rootDir, []string{filepath.Join("internal", "core")})
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{
		rootDir:  searchableIndex(),
		childDir: searchableIndexForMetadataPath(childDir, "go.mod"),
	}}
	service := newSearchCommandServiceForFunctionalTests(tree, out, fakeSearchFileReader{files: map[string]string{}}, repo)

	// Act
	require.NoError(t, service.RunWithOptions("", search.Options{PathQuery: "internal core"}))

	// Assert
	assert.Equal(t, "internal/core/go.mod", stripANSICodes(out.lines[1]))
}

func TestSearchCommandService_RunWithOptions_PathWildcardSuffixFilter(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	childDir := filepath.Join(rootDir, "internal", "core")
	tree := searchTreeWithIndexes(rootDir, []string{filepath.Join("internal", "core")})
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{
		rootDir:  searchableIndex(),
		childDir: searchableIndexForMetadataPath(childDir, "go.mod"),
	}}
	service := newSearchCommandServiceForFunctionalTests(tree, out, fakeSearchFileReader{files: map[string]string{}}, repo)

	// Act
	require.NoError(t, service.RunWithOptions("", search.Options{PathQuery: "*core"}))

	// Assert
	assert.Equal(t, "internal/core/go.mod", stripANSICodes(out.lines[1]))
}

func TestSearchCommandService_RunWithOptions_ExtensionFilter(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{filepath.Join(rootDir, "AGENTS.md"): "idx"}}
	service := newSearchCommandServiceForFunctionalTests(tree, out, fileReader, repo)

	// Act
	require.NoError(t, service.RunWithOptions("idx", search.Options{ExtensionQuery: "md"}))

	// Assert
	require.GreaterOrEqual(t, len(out.lines), 2)
	assert.Equal(t, "./AGENTS.md", stripANSICodes(out.lines[1]))
}

func TestSearchCommandService_RunWithOptions_ExplainShowsScoreInTextOutput(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{filepath.Join(rootDir, "go.mod"): "module idx"}}
	service := newSearchCommandServiceForFunctionalTests(tree, out, fileReader, repo)

	// Act
	require.NoError(t, service.RunWithOptions("module idx", search.Options{Explain: true}))

	// Assert
	assert.Equal(t, "./go.mod (score: 1.0000)", stripANSICodes(out.lines[1]))
}

func TestSearchCommandService_RunWithOptions_JSONOmitsScoreByDefault(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{filepath.Join(rootDir, "go.mod"): "module idx"}}
	service := newSearchCommandServiceForFunctionalTests(tree, out, fileReader, repo)

	// Act
	require.NoError(t, service.RunWithOptions("module idx", search.Options{Format: search.OutputJSON}))

	// Assert
	var response map[string]any
	require.NoError(t, json.Unmarshal([]byte(out.lines[0]), &response))
	results := response["results"].([]any)
	first := results[0].(map[string]any)
	_, hasScore := first["score"]
	assert.False(t, hasScore, "expected score to be omitted by default")
}

func TestSearchCommandService_RunWithOptions_JSONIncludesScoreWithExplain(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{filepath.Join(rootDir, "go.mod"): "module idx"}}
	service := newSearchCommandServiceForFunctionalTests(tree, out, fileReader, repo)

	// Act
	require.NoError(t, service.RunWithOptions("module idx", search.Options{Format: search.OutputJSON, Explain: true}))

	// Assert
	var response map[string]any
	require.NoError(t, json.Unmarshal([]byte(out.lines[0]), &response))
	results := response["results"].([]any)
	first := results[0].(map[string]any)
	_, hasScore := first["score"]
	assert.True(t, hasScore, "expected score present when explain enabled")

	_ = strings.Contains // keep import used
}
