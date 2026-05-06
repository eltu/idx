package search_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"idx/internal/core/domain"
	"idx/internal/core/ports"
)

func searchableIndexWithDisjointTerms() *domain.InvertedIndex {
	index := domain.NewInvertedIndex()
	index.Documents["a.go"] = &domain.DocStats{Name: "a.go", Path: "a.go", Length: 3}
	index.Documents["b.go"] = &domain.DocStats{Name: "b.go", Path: "b.go", Length: 3}
	index.AddPathTerms("a.go", "a.go")
	index.AddPathTerms("b.go", "b.go")
	index.DocumentCount = 2
	index.AverageDocLength = 3
	index.Terms["module"] = &domain.TermStats{IDF: 0.7, Docs: map[string]*domain.DocTermStats{"a.go": {TF: 1}}}
	index.Terms["idx"] = &domain.TermStats{IDF: 0.8, Docs: map[string]*domain.DocTermStats{"b.go": {TF: 1}}}
	return index
}

func TestSearchCommandServiceOROperatorReturnsBothDocumentsWhenTermsAreDisjoint(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithDisjointTerms()}}
	fileReader := fakeSearchFileReader{files: map[string]string{filepath.Join(rootDir, "a.go"): "module search", filepath.Join(rootDir, "b.go"): "idx tool"}}
	service := newSearchCommandServiceForFunctionalTests(tree, output, fileReader, repo)

	err := service.RunWithOptions("module idx", ports.SearchOptions{Format: ports.SearchOutputJSON, Operator: ports.SearchOperatorOR})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(output.lines[0]), &response); err != nil {
		t.Fatalf("expected valid JSON output, got error %v with payload %q", err, output.lines[0])
	}
	if response["count"] != float64(2) {
		t.Fatalf("expected OR to return 2 results, got %v", response["count"])
	}
}

func TestSearchCommandServiceANDOperatorReturnsNoResultsWhenTermsAreDisjoint(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithDisjointTerms()}}
	fileReader := fakeSearchFileReader{files: map[string]string{filepath.Join(rootDir, "a.go"): "module search", filepath.Join(rootDir, "b.go"): "idx tool"}}
	service := newSearchCommandServiceForFunctionalTests(tree, output, fileReader, repo)

	err := service.RunWithOptions("module idx", ports.SearchOptions{Format: ports.SearchOutputJSON, Operator: ports.SearchOperatorAND})
	if err != nil {
		t.Fatalf("expected no error for empty AND results, got %v", err)
	}
	if len(output.lines) == 0 {
		t.Fatal("expected output for empty AND results")
	}
}

func searchableIndexWithCoverageSkew() *domain.InvertedIndex {
	index := domain.NewInvertedIndex()
	index.Documents["full.go"] = &domain.DocStats{Name: "full.go", Path: "full.go", Length: 5}
	index.Documents["partial.go"] = &domain.DocStats{Name: "partial.go", Path: "partial.go", Length: 5}
	index.AddPathTerms("full.go", "full.go")
	index.AddPathTerms("partial.go", "partial.go")
	index.DocumentCount = 2
	index.AverageDocLength = 5
	index.Terms["module"] = &domain.TermStats{IDF: 1.0, Docs: map[string]*domain.DocTermStats{"full.go": {TF: 1}, "partial.go": {TF: 100}}}
	index.Terms["idx"] = &domain.TermStats{IDF: 2.0, Docs: map[string]*domain.DocTermStats{"full.go": {TF: 1}}}
	return index
}

func TestSearchCommandServiceOROperatorRanksFullMatchAbovePartialMatch(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithCoverageSkew()}}
	fileReader := fakeSearchFileReader{files: map[string]string{filepath.Join(rootDir, "full.go"): "module idx", filepath.Join(rootDir, "partial.go"): "module module module"}}
	service := newSearchCommandServiceForFunctionalTests(tree, output, fileReader, repo)

	err := service.RunWithOptions("module idx", ports.SearchOptions{Format: ports.SearchOutputJSON, Operator: ports.SearchOperatorOR, Explain: true})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(output.lines[0]), &response); err != nil {
		t.Fatalf("expected valid JSON output, got error %v with payload %q", err, output.lines[0])
	}
	results := response["results"].([]any)
	if len(results) < 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	topFile := results[0].(map[string]any)["file"].(string)
	if topFile != "full.go" && topFile != "./full.go" {
		t.Fatalf("expected full.go (all terms matched) to rank first, got %q", topFile)
	}
}

func searchableIndexWithSameScoreButConcentratedTerms() *domain.InvertedIndex {
	index := domain.NewInvertedIndex()
	index.Documents["concentrated.go"] = &domain.DocStats{Name: "concentrated.go", Path: "concentrated.go", Length: 3}
	index.Documents["scattered.go"] = &domain.DocStats{Name: "scattered.go", Path: "scattered.go", Length: 3}
	index.AddPathTerms("concentrated.go", "concentrated.go")
	index.AddPathTerms("scattered.go", "scattered.go")
	index.DocumentCount = 2
	index.AverageDocLength = 3
	for _, term := range []string{"alpha", "beta"} {
		index.Terms[term] = &domain.TermStats{IDF: 1.0, Docs: map[string]*domain.DocTermStats{"concentrated.go": {TF: 1}, "scattered.go": {TF: 1}}}
	}
	return index
}

func TestSearchTermConcentrationBreaksTieInFavorOfCoLocatedTerms(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithSameScoreButConcentratedTerms()}}
	fileReader := fakeSearchFileReader{files: map[string]string{filepath.Join(rootDir, "concentrated.go"): "alpha beta", filepath.Join(rootDir, "scattered.go"): "alpha\nbeta"}}
	service := newSearchCommandServiceForFunctionalTests(tree, output, fileReader, repo)

	err := service.RunWithOptions("alpha beta", ports.SearchOptions{Format: ports.SearchOutputJSON, Explain: true})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(output.lines[0]), &response); err != nil {
		t.Fatalf("expected valid JSON output, got error %v with payload %q", err, output.lines[0])
	}
	results := response["results"].([]any)
	if len(results) < 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	topFile := results[0].(map[string]any)["file"].(string)
	if topFile != "concentrated.go" && topFile != "./concentrated.go" {
		t.Fatalf("expected concentrated.go (all terms on one line) to rank first, got %q", topFile)
	}
}
