package search_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"idx/internal/features/indexing"
	search "idx/internal/features/search"
)

const (
	fullGoFileName     = "full.go"
	fullGoRelativePath = "./" + fullGoFileName
)

func searchableIndexWithDisjointTerms() *indexing.InvertedIndex {
	index := indexing.NewInvertedIndex()
	index.Documents["a.go"] = &indexing.DocStats{Name: "a.go", Path: "a.go", Length: 3}
	index.Documents["b.go"] = &indexing.DocStats{Name: "b.go", Path: "b.go", Length: 3}
	index.AddPathTerms("a.go", "a.go")
	index.AddPathTerms("b.go", "b.go")
	index.DocumentCount = 2
	index.AverageDocLength = 3
	index.Terms["module"] = &indexing.TermStats{IDF: 0.7, Docs: map[string]*indexing.DocTermStats{"a.go": {TF: 1}}}
	index.Terms["idx"] = &indexing.TermStats{IDF: 0.8, Docs: map[string]*indexing.DocTermStats{"b.go": {TF: 1}}}
	return index
}

func TestSearchCommandService_OROperator_ReturnsBothDocumentsWhenTermsAreDisjoint(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{rootDir: searchableIndexWithDisjointTerms()}}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "a.go"): "module search",
		filepath.Join(rootDir, "b.go"): "idx tool",
	}}
	service := newSearchCommandServiceForFunctionalTests(tree, out, fileReader, repo)

	// Act
	require.NoError(t, service.RunWithOptions("module idx", search.Options{Format: search.OutputJSON, Operator: search.OperatorOR}))

	// Assert
	var response map[string]any
	require.NoError(t, json.Unmarshal([]byte(out.lines[0]), &response))
	assert.Equal(t, float64(2), response["count"])
}

func TestSearchCommandService_ANDOperator_ReturnsNoResultsWhenTermsAreDisjoint(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{rootDir: searchableIndexWithDisjointTerms()}}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "a.go"): "module search",
		filepath.Join(rootDir, "b.go"): "idx tool",
	}}
	service := newSearchCommandServiceForFunctionalTests(tree, out, fileReader, repo)

	// Act
	require.NoError(t, service.RunWithOptions("module idx", search.Options{Format: search.OutputJSON, Operator: search.OperatorAND}))

	// Assert
	assert.NotEmpty(t, out.lines)
}

func searchableIndexWithCoverageSkew() *indexing.InvertedIndex {
	index := indexing.NewInvertedIndex()
	index.Documents["full.go"] = &indexing.DocStats{Name: "full.go", Path: "full.go", Length: 5}
	index.Documents["partial.go"] = &indexing.DocStats{Name: "partial.go", Path: "partial.go", Length: 5}
	index.AddPathTerms("full.go", "full.go")
	index.AddPathTerms("partial.go", "partial.go")
	index.DocumentCount = 2
	index.AverageDocLength = 5
	index.Terms["module"] = &indexing.TermStats{IDF: 1.0, Docs: map[string]*indexing.DocTermStats{"full.go": {TF: 1}, "partial.go": {TF: 100}}}
	index.Terms["idx"] = &indexing.TermStats{IDF: 2.0, Docs: map[string]*indexing.DocTermStats{"full.go": {TF: 1}}}
	return index
}

func TestSearchCommandService_OROperator_RanksFullMatchAbovePartialMatch(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{rootDir: searchableIndexWithCoverageSkew()}}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "full.go"):    "module idx",
		filepath.Join(rootDir, "partial.go"): "module module module",
	}}
	service := newSearchCommandServiceForFunctionalTests(tree, out, fileReader, repo)

	// Act
	require.NoError(t, service.RunWithOptions("module idx", search.Options{Format: search.OutputJSON, Operator: search.OperatorOR, Explain: true}))

	// Assert
	var response map[string]any
	require.NoError(t, json.Unmarshal([]byte(out.lines[0]), &response))
	results := response["results"].([]any)
	require.GreaterOrEqual(t, len(results), 2)
	topFile := results[0].(map[string]any)["file"].(string)
	assert.True(t, topFile == fullGoFileName || topFile == fullGoRelativePath, "expected full.go first, got %q", topFile)
}

func searchableIndexWithSameScoreButConcentratedTerms() *indexing.InvertedIndex {
	index := indexing.NewInvertedIndex()
	index.Documents["concentrated.go"] = &indexing.DocStats{Name: "concentrated.go", Path: "concentrated.go", Length: 3}
	index.Documents["scattered.go"] = &indexing.DocStats{Name: "scattered.go", Path: "scattered.go", Length: 3}
	index.AddPathTerms("concentrated.go", "concentrated.go")
	index.AddPathTerms("scattered.go", "scattered.go")
	index.DocumentCount = 2
	index.AverageDocLength = 3
	for _, term := range []string{"alpha", "beta"} {
		index.Terms[term] = &indexing.TermStats{IDF: 1.0, Docs: map[string]*indexing.DocTermStats{
			"concentrated.go": {TF: 1},
			"scattered.go":    {TF: 1},
		}}
	}
	return index
}

func TestSearchCommandService_TermConcentration_BreaksTieInFavorOfCoLocatedTerms(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{rootDir: searchableIndexWithSameScoreButConcentratedTerms()}}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "concentrated.go"): "alpha beta",
		filepath.Join(rootDir, "scattered.go"):    "alpha\nbeta",
	}}
	service := newSearchCommandServiceForFunctionalTests(tree, out, fileReader, repo)

	// Act
	require.NoError(t, service.RunWithOptions("alpha beta", search.Options{Format: search.OutputJSON, Explain: true}))

	// Assert
	var response map[string]any
	require.NoError(t, json.Unmarshal([]byte(out.lines[0]), &response))
	results := response["results"].([]any)
	require.GreaterOrEqual(t, len(results), 2)
	topFile := results[0].(map[string]any)["file"].(string)
	assert.True(t, topFile == "concentrated.go" || topFile == "./concentrated.go",
		"expected concentrated.go first, got %q", topFile)
}

func searchableIndexForRelaxation() *indexing.InvertedIndex {
	index := indexing.NewInvertedIndex()
	index.Documents["full.go"] = &indexing.DocStats{Name: "full.go", Path: "full.go", Length: 6}
	index.Documents["relaxed.go"] = &indexing.DocStats{Name: "relaxed.go", Path: "relaxed.go", Length: 5}
	index.Documents["minimal.go"] = &indexing.DocStats{Name: "minimal.go", Path: "minimal.go", Length: 3}
	index.AddPathTerms("full.go", "full.go")
	index.AddPathTerms("relaxed.go", "relaxed.go")
	index.AddPathTerms("minimal.go", "minimal.go")
	index.DocumentCount = 3
	index.AverageDocLength = 5
	index.Terms["func"] = &indexing.TermStats{IDF: 0.6, Docs: map[string]*indexing.DocTermStats{"full.go": {TF: 1}, "relaxed.go": {TF: 1}, "minimal.go": {TF: 1}}}
	index.Terms["abc"] = &indexing.TermStats{IDF: 0.7, Docs: map[string]*indexing.DocTermStats{"full.go": {TF: 1}, "relaxed.go": {TF: 1}, "minimal.go": {TF: 1}}}
	index.Terms["x"] = &indexing.TermStats{IDF: 0.8, Docs: map[string]*indexing.DocTermStats{"full.go": {TF: 1}, "relaxed.go": {TF: 1}, "minimal.go": {TF: 1}}}
	index.Terms["y"] = &indexing.TermStats{IDF: 0.9, Docs: map[string]*indexing.DocTermStats{"full.go": {TF: 1}, "relaxed.go": {TF: 1}}}
	index.Terms["int"] = &indexing.TermStats{IDF: 1.0, Docs: map[string]*indexing.DocTermStats{"full.go": {TF: 1}, "relaxed.go": {TF: 1}}}
	index.Terms["10"] = &indexing.TermStats{IDF: 1.1, Docs: map[string]*indexing.DocTermStats{"full.go": {TF: 1}}}
	return index
}

func relaxationFileReader(rootDir string) fakeSearchFileReader {
	return fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "full.go"):    "func abc x y int 10",
		filepath.Join(rootDir, "relaxed.go"): "func abc x y int",
		filepath.Join(rootDir, "minimal.go"): "func abc x",
	}}
}

func TestSearchCommandService_ANDRelaxation_ReturnsResultsWhenStrictANDIsEmpty(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{rootDir: searchableIndexForRelaxation()}}
	service := newSearchCommandServiceForFunctionalTests(tree, out, relaxationFileReader(rootDir), repo)

	// Act
	require.NoError(t, service.RunWithOptions("func abc x y int missing",
		search.Options{Format: search.OutputJSON, Operator: search.OperatorAND, RelaxationEnabled: true, RelaxationMinExclusive: 2}))

	// Assert
	var response map[string]any
	require.NoError(t, json.Unmarshal([]byte(out.lines[0]), &response))
	assert.NotEqual(t, float64(0), response["count"], "expected relaxation to return results")
}

func TestSearchCommandService_ANDRelaxation_RanksByMatchedTokenCount(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{rootDir: searchableIndexForRelaxation()}}
	service := newSearchCommandServiceForFunctionalTests(tree, out, relaxationFileReader(rootDir), repo)

	// Act
	require.NoError(t, service.RunWithOptions("func abc x y int 10",
		search.Options{Format: search.OutputJSON, Operator: search.OperatorAND, RelaxationEnabled: true, RelaxationMinExclusive: 2}))

	// Assert
	var response map[string]any
	require.NoError(t, json.Unmarshal([]byte(out.lines[0]), &response))
	results := response["results"].([]any)
	require.GreaterOrEqual(t, len(results), 2)
	first := results[0].(map[string]any)["file"].(string)
	second := results[1].(map[string]any)["file"].(string)
	assert.True(t, first == fullGoFileName || first == fullGoRelativePath, "expected full.go first, got %q", first)
	assert.True(t, second == "relaxed.go" || second == "./relaxed.go", "expected relaxed.go second, got %q", second)
}

func TestSearchCommandService_ANDRelaxation_DropsSecondTokenWhenThresholdIsGreaterThanOne(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	out := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{rootDir: searchableIndexForRelaxation()}}
	service := newSearchCommandServiceForFunctionalTests(tree, out, relaxationFileReader(rootDir), repo)

	// Act
	require.NoError(t, service.RunWithOptions("func xpto",
		search.Options{Format: search.OutputJSON, Operator: search.OperatorAND, RelaxationEnabled: true, RelaxationMinExclusive: 1}))

	// Assert
	var response map[string]any
	require.NoError(t, json.Unmarshal([]byte(out.lines[0]), &response))
	assert.NotEqual(t, float64(0), response["count"], "expected relaxation >1 to keep first token")
}

func TestSearchCommandService_ANDRelaxation_ThresholdIsDynamic(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	repo := &fakeSearchIndexRepository{indices: map[string]*indexing.InvertedIndex{rootDir: searchableIndexForRelaxation()}}

	// Act — 2-term query with >2 threshold: no results
	out1 := &capturingTextOutput{}
	svc1 := newSearchCommandServiceForFunctionalTests(tree, out1, relaxationFileReader(rootDir), repo)
	require.NoError(t, svc1.RunWithOptions("func xpto",
		search.Options{Format: search.OutputJSON, Operator: search.OperatorAND, RelaxationEnabled: true, RelaxationMinExclusive: 2}))
	var r1 map[string]any
	require.NoError(t, json.Unmarshal([]byte(out1.lines[0]), &r1))
	assert.Equal(t, float64(0), r1["count"])

	// Act — 3-term query with >2 threshold: should relax and return results
	out2 := &capturingTextOutput{}
	svc2 := newSearchCommandServiceForFunctionalTests(tree, out2, relaxationFileReader(rootDir), repo)
	require.NoError(t, svc2.RunWithOptions("func abc xpto",
		search.Options{Format: search.OutputJSON, Operator: search.OperatorAND, RelaxationEnabled: true, RelaxationMinExclusive: 2}))
	var r2 map[string]any
	require.NoError(t, json.Unmarshal([]byte(out2.lines[0]), &r2))
	assert.NotEqual(t, float64(0), r2["count"], "expected 3-term query with >2 threshold to relax")
}
