package search_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"idx/internal/core/domain"
	"idx/internal/core/ports"
)

func TestSearchCommandServiceRunWithOptionsReturnsJSONOutput(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{filepath.Join(rootDir, "go.mod"): "module idx"}}
	service := newSearchCommandServiceForFunctionalTests(tree, output, fileReader, repo)

	err := service.RunWithOptions("module idx", ports.SearchOptions{Format: ports.SearchOutputJSON})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(output.lines[0]), &response); err != nil {
		t.Fatalf("expected valid JSON output, got error %v with payload %q", err, output.lines[0])
	}

	if response["count"] != float64(1) {
		t.Fatalf("expected count 1, got %v", response["count"])
	}
}

func TestSearchCommandServiceRunWithOptionsReturnsPrettyJSONOutput(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{filepath.Join(rootDir, "go.mod"): "module idx"}}
	service := newSearchCommandServiceForFunctionalTests(tree, output, fileReader, repo)

	err := service.RunWithOptions("module idx", ports.SearchOptions{Format: ports.SearchOutputJSON, PrettyJSON: true})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !strings.Contains(output.lines[0], "\n") {
		t.Fatalf("expected pretty JSON with line breaks, got %q", output.lines[0])
	}
}

func TestSearchCommandServiceRunWithOptionsIncludesContextLines(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{filepath.Join(rootDir, "go.mod"): "alpha\nmodule idx\nomega"}}
	service := newSearchCommandServiceForFunctionalTests(tree, output, fileReader, repo)

	err := service.RunWithOptions("module idx", ports.SearchOptions{Context: 1})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if stripANSICodes(output.lines[2]) != "├── 1: alpha" {
		t.Fatalf("expected first context line, got %q", output.lines[2])
	}
}

func TestSearchCommandServiceRunWithOptionsMatchesOnlyFiltersContextLines(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{filepath.Join(rootDir, "go.mod"): "alpha\nmodule idx\nomega"}}
	service := newSearchCommandServiceForFunctionalTests(tree, output, fileReader, repo)

	err := service.RunWithOptions("module idx", ports.SearchOptions{Format: ports.SearchOutputJSON, Context: 1, MatchesOnly: true})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(output.lines[0]), &response); err != nil {
		t.Fatalf("expected valid JSON output, got error %v with payload %q", err, output.lines[0])
	}

	results := response["results"].([]any)
	file := results[0].(map[string]any)
	matches := file["matches"].([]any)
	if len(matches) != 1 {
		t.Fatalf("expected one matched line after filtering context, got %d", len(matches))
	}
}

func TestSearchCommandServiceRunWithOptionsSizeRestrictsResultCount(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "guide.md"):  "go search guide",
		filepath.Join(rootDir, "readme.md"): "go content\nsearch topic",
	}}
	service := newSearchCommandServiceForFunctionalTests(tree, output, fileReader, repo)

	err := service.RunWithOptions("go search", ports.SearchOptions{Format: ports.SearchOutputJSON, Size: 1})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(output.lines[0]), &response); err != nil {
		t.Fatalf("expected valid JSON output, got error %v with payload %q", err, output.lines[0])
	}
	results := response["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("expected one file result with size, got %d", len(results))
	}
}

func TestSearchCommandServiceRunWithOptionsFromAndSizePaginateResults(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "guide.md"):  "go search guide",
		filepath.Join(rootDir, "readme.md"): "go content\nsearch topic",
	}}
	service := newSearchCommandServiceForFunctionalTests(tree, output, fileReader, repo)

	err := service.RunWithOptions("go search", ports.SearchOptions{Format: ports.SearchOutputJSON, From: 1, Size: 1})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(output.lines[0]), &response); err != nil {
		t.Fatalf("expected valid JSON output, got error %v with payload %q", err, output.lines[0])
	}
	payload := response["results"].([]any)[0].(map[string]any)
	if payload["file"] != "./readme.md" {
		t.Fatalf("expected second-ranked file ./readme.md for from=1,size=1, got %v", payload["file"])
	}
}

func TestSearchCommandServiceDisplaysMatchCountInTextFormat(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "guide.md"):  "go search guide",
		filepath.Join(rootDir, "readme.md"): "go content\nsearch topic",
	}}
	service := newSearchCommandServiceForFunctionalTests(tree, output, fileReader, repo)

	err := service.RunWithOptions("go search", ports.SearchOptions{Format: ports.SearchOutputText})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !strings.Contains(output.lines[0], "Found 2 file(s)") {
		t.Fatalf("expected match count header, got %q", output.lines[0])
	}
}

func TestSearchCommandServiceDisplaysMatchCountWithPaginationInTextFormat(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "guide.md"):  "go search guide",
		filepath.Join(rootDir, "readme.md"): "go content\nsearch topic",
	}}
	service := newSearchCommandServiceForFunctionalTests(tree, output, fileReader, repo)

	err := service.RunWithOptions("go search", ports.SearchOptions{Format: ports.SearchOutputText, Size: 1})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !strings.Contains(output.lines[0], "showing 1") {
		t.Fatalf("expected pagination info in header, got %q", output.lines[0])
	}
}

func TestSearchCommandServiceRunWithOptionsFilesOnlyReturnsPathsOnly(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "guide.md"):  "go search guide",
		filepath.Join(rootDir, "readme.md"): "go content\nsearch topic",
	}}
	service := newSearchCommandServiceForFunctionalTests(tree, output, fileReader, repo)

	err := service.RunWithOptions("go search", ports.SearchOptions{Format: ports.SearchOutputText, FilesOnly: true})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(output.lines) != 3 {
		t.Fatalf("expected 3 output lines, got %d: %v", len(output.lines), output.lines)
	}
}

func TestSearchCommandServiceRunWithOptionsFilesOnlyReturnsJSONArray(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "guide.md"):  "go search guide",
		filepath.Join(rootDir, "readme.md"): "go content\nsearch topic",
	}}
	service := newSearchCommandServiceForFunctionalTests(tree, output, fileReader, repo)

	err := service.RunWithOptions("go search", ports.SearchOptions{Format: ports.SearchOutputJSON, FilesOnly: true})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var payload []string
	if err := json.Unmarshal([]byte(output.lines[0]), &payload); err != nil {
		t.Fatalf("expected valid JSON array output, got error %v with payload %q", err, output.lines[0])
	}
	if len(payload) != 2 {
		t.Fatalf("expected 2 file paths in JSON, got %d: %v", len(payload), payload)
	}
}

func TestSearchCommandServiceRunWithOptionsFilesOnlyWithJSONPretty(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "guide.md"):  "go search guide",
		filepath.Join(rootDir, "readme.md"): "go content\nsearch topic",
	}}
	service := newSearchCommandServiceForFunctionalTests(tree, output, fileReader, repo)

	err := service.RunWithOptions("go search", ports.SearchOptions{Format: ports.SearchOutputJSON, FilesOnly: true, PrettyJSON: true})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !strings.Contains(output.lines[0], "\n") {
		t.Fatalf("expected pretty JSON with line breaks, got %q", output.lines[0])
	}
}

func TestSearchCommandServiceRunWithOptionsSupportsMetadataOnlyPathFilter(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	childDir := filepath.Join(rootDir, "internal", "core")
	tree := searchTreeWithIndexes(rootDir, []string{filepath.Join("internal", "core")})
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*domain.InvertedIndex{
		rootDir:  searchableIndex(),
		childDir: searchableIndexForMetadataPath(childDir, "go.mod"),
	}}
	service := newSearchCommandServiceForFunctionalTests(tree, output, fakeSearchFileReader{files: map[string]string{}}, repo)

	err := service.RunWithOptions("", ports.SearchOptions{PathQuery: "internal core"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if stripANSICodes(output.lines[1]) != "internal/core/go.mod" {
		t.Fatalf("expected metadata-only path result, got %q", output.lines[1])
	}
}

func TestSearchCommandServiceRunWithOptionsSupportsPathWildcardSuffixFilter(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	childDir := filepath.Join(rootDir, "internal", "core")
	tree := searchTreeWithIndexes(rootDir, []string{filepath.Join("internal", "core")})
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*domain.InvertedIndex{
		rootDir:  searchableIndex(),
		childDir: searchableIndexForMetadataPath(childDir, "go.mod"),
	}}
	service := newSearchCommandServiceForFunctionalTests(tree, output, fakeSearchFileReader{files: map[string]string{}}, repo)

	err := service.RunWithOptions("", ports.SearchOptions{PathQuery: "*core"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if stripANSICodes(output.lines[1]) != "internal/core/go.mod" {
		t.Fatalf("expected internal/core/go.mod with suffix wildcard, got %q", output.lines[1])
	}
}

func TestSearchCommandServiceRunWithOptionsExplainShowsScoreInTextOutput(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{filepath.Join(rootDir, "go.mod"): "module idx"}}
	service := newSearchCommandServiceForFunctionalTests(tree, output, fileReader, repo)

	err := service.RunWithOptions("module idx", ports.SearchOptions{Explain: true})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if stripANSICodes(output.lines[1]) != "./go.mod (score: 1.0000)" {
		t.Fatalf("expected score in text output with explain, got %q", output.lines[1])
	}
}

func TestSearchCommandServiceRunWithOptionsJSONOmitsScoreByDefault(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{filepath.Join(rootDir, "go.mod"): "module idx"}}
	service := newSearchCommandServiceForFunctionalTests(tree, output, fileReader, repo)

	err := service.RunWithOptions("module idx", ports.SearchOptions{Format: ports.SearchOutputJSON})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(output.lines[0]), &response); err != nil {
		t.Fatalf("expected valid JSON output, got error %v with payload %q", err, output.lines[0])
	}

	results := response["results"].([]any)
	first := results[0].(map[string]any)
	if _, exists := first["score"]; exists {
		t.Fatalf("expected score to be omitted by default, got %v", first["score"])
	}
}

func TestSearchCommandServiceRunWithOptionsJSONIncludesScoreWithExplain(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{filepath.Join(rootDir, "go.mod"): "module idx"}}
	service := newSearchCommandServiceForFunctionalTests(tree, output, fileReader, repo)

	err := service.RunWithOptions("module idx", ports.SearchOptions{Format: ports.SearchOutputJSON, Explain: true})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(output.lines[0]), &response); err != nil {
		t.Fatalf("expected valid JSON output, got error %v with payload %q", err, output.lines[0])
	}

	results := response["results"].([]any)
	first := results[0].(map[string]any)
	if _, exists := first["score"]; !exists {
		t.Fatal("expected score to be present when explain is enabled")
	}
}
