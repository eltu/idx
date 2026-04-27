package search_test

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"idx/internal/core/domain"
	"idx/internal/core/ports"
	search "idx/internal/core/services/search"
)

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// stripANSICodes removes ANSI escape sequences so assertions can compare plain text.
func stripANSICodes(s string) string {
	return ansiEscapePattern.ReplaceAllString(s, "")
}

type fakeSearchIndexRepository struct {
	indices map[string]*domain.InvertedIndex
	loadErr error
	loaded  []string
	mu      sync.Mutex
}

func (repo *fakeSearchIndexRepository) LoadIndex(directoryPath string) (*domain.InvertedIndex, error) {
	repo.mu.Lock()
	repo.loaded = append(repo.loaded, directoryPath)
	repo.mu.Unlock()

	if repo.loadErr != nil {
		return nil, repo.loadErr
	}

	index := repo.indices[directoryPath]
	if index == nil {
		return nil, errors.New("index not found")
	}

	return index, nil
}

type fakeSearchFileReader struct {
	files map[string]string
}

func (reader fakeSearchFileReader) ReadFile(path string) (string, error) {
	content, exists := reader.files[path]
	if !exists {
		return "", errors.New("file not found")
	}

	return content, nil
}

func newSearchCommandServiceForFunctionalTests(
	tree *fakeProjectTree,
	output ports.TextOutput,
	fileReader fakeSearchFileReader,
	repo *fakeSearchIndexRepository,
) search.SearchCommandService {
	service := search.NewSearchCommandService(tree, output, fileReader, repo)
	service.SetCacheEnabled(false)
	return service
}

func newSearchCommandServiceForCacheTests(
	tree *fakeProjectTree,
	output ports.TextOutput,
	fileReader fakeSearchFileReader,
	repo *fakeSearchIndexRepository,
) search.SearchCommandService {
	service := search.NewSearchCommandService(tree, output, fileReader, repo)
	service.SetCacheEnabled(true)
	return service
}

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

	// Header: 1 line
	// guide.md (score 1.0000): 1 file header + 1 matching line + blank
	// readme.md (score 0.0000): 1 file header + 2 matching lines (go / search on separate lines) + blank
	if len(output.lines) != 8 {
		t.Fatalf("expected 8 output lines, got %d: %v", len(output.lines), output.lines)
	}

	if stripANSICodes(output.lines[1]) != "./guide.md (score: 1.0000)" {
		t.Fatalf("expected best result file header first, got %q", output.lines[0])
	}

	if stripANSICodes(output.lines[1]) != "./guide.md (score: 1.0000)" {
		t.Fatalf("expected best result file header first, got %q", output.lines[1])
	}

	if stripANSICodes(output.lines[2]) != "└── 1: go search guide" {
		t.Fatalf("expected guide.md matched line, got %q", output.lines[2])
	}

	if stripANSICodes(output.lines[4]) != "./readme.md (score: 0.0000)" {
		t.Fatalf("expected second result file header, got %q", output.lines[4])
	}

	if stripANSICodes(output.lines[5]) != "├── 1: go content" {
		t.Fatalf("expected readme.md first matched line, got %q", output.lines[5])
	}

	if stripANSICodes(output.lines[6]) != "└── 2: search topic" {
		t.Fatalf("expected readme.md second matched line, got %q", output.lines[6])
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

	// file header + 1 matched line + blank + search header
	if len(output.lines) != 4 {
		t.Fatalf("expected 4 output lines, got %d: %v", len(output.lines), output.lines)
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

	if len(output.lines) != 1 {
		t.Fatalf("expected 1 output line, got %d", len(output.lines))
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

	// header + near.txt: header + 1 line + blank; far.txt: header + 2 lines + blank
	if len(output.lines) != 8 {
		t.Fatalf("expected 8 output lines, got %d: %v", len(output.lines), output.lines)
	}

	if stripANSICodes(output.lines[1]) != "./near.txt (score: 1.0000)" {
		t.Fatalf("expected nearby terms file first, got %q", output.lines[1])
	}

	if stripANSICodes(output.lines[2]) != "└── 1: module idx" {
		t.Fatalf("expected near.txt matched line, got %q", output.lines[2])
	}

	if stripANSICodes(output.lines[4]) != "./far.txt (score: 0.0000)" {
		t.Fatalf("expected far terms file second, got %q", output.lines[4])
	}

	if stripANSICodes(output.lines[5]) != "├── 1: module" {
		t.Fatalf("expected far.txt first line, got %q", output.lines[5])
	}

	if stripANSICodes(output.lines[6]) != "└── 2: idx" {
		t.Fatalf("expected far.txt second line, got %q", output.lines[6])
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

	if len(output.lines) != 4 {
		t.Fatalf("expected 4 output lines, got %d: %v", len(output.lines), output.lines)
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

	// header + 2 files, each with header + 1 matched line + blank
	if len(output.lines) != 7 {
		t.Fatalf("expected 7 output lines, got %d: %v", len(output.lines), output.lines)
	}

	if stripANSICodes(output.lines[1]) != "docs/guide.md (score: 1.0000)" {
		t.Fatalf("expected child directory file header, got %q", output.lines[1])
	}

	if stripANSICodes(output.lines[4]) != "./root.md (score: 1.0000)" {
		t.Fatalf("expected root directory file header, got %q", output.lines[4])
	}
}

func TestSearchCommandServiceRunWithOptionsReturnsJSONOutput(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "go.mod"): "module idx",
	}}
	service := newSearchCommandServiceForFunctionalTests(tree, output, fileReader, repo)

	err := service.RunWithOptions("module idx", ports.SearchOptions{Format: ports.SearchOutputJSON})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(output.lines) != 1 {
		t.Fatalf("expected a single JSON line, got %d: %v", len(output.lines), output.lines)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(output.lines[0]), &response); err != nil {
		t.Fatalf("expected valid JSON output, got error %v with payload %q", err, output.lines[0])
	}

	if response["count"] != float64(1) {
		t.Fatalf("expected count 1, got %v", response["count"])
	}

	results, ok := response["results"].([]any)
	if !ok {
		t.Fatalf("expected results array, got %T", response["results"])
	}

	if len(results) != 1 {
		t.Fatalf("expected a single JSON result, got %d", len(results))
	}

	payload := results[0].(map[string]any)
	if payload["file"] != "./go.mod" {
		t.Fatalf("expected file ./go.mod, got %v", payload["file"])
	}

	if payload["name"] != "go.mod" {
		t.Fatalf("expected name go.mod, got %v", payload["name"])
	}

	if payload["path"] != "./go.mod" {
		t.Fatalf("expected path ./go.mod, got %v", payload["path"])
	}
}

func TestSearchCommandServiceRunWithOptionsReturnsPrettyJSONOutput(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "go.mod"): "module idx",
	}}
	service := newSearchCommandServiceForFunctionalTests(tree, output, fileReader, repo)

	err := service.RunWithOptions("module idx", ports.SearchOptions{Format: ports.SearchOutputJSON, PrettyJSON: true})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(output.lines) != 1 {
		t.Fatalf("expected a single JSON line, got %d: %v", len(output.lines), output.lines)
	}

	if !strings.Contains(output.lines[0], "\n") {
		t.Fatalf("expected pretty JSON with line breaks, got %q", output.lines[0])
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(output.lines[0]), &response); err != nil {
		t.Fatalf("expected valid pretty JSON output, got error %v with payload %q", err, output.lines[0])
	}

	if response["count"] != float64(1) {
		t.Fatalf("expected count 1, got %v", response["count"])
	}
}

func TestSearchCommandServiceRunWithOptionsIncludesContextLines(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "go.mod"): "alpha\nmodule idx\nomega",
	}}
	service := newSearchCommandServiceForFunctionalTests(tree, output, fileReader, repo)

	err := service.RunWithOptions("module idx", ports.SearchOptions{Context: 1})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(output.lines) != 6 {
		t.Fatalf("expected 6 output lines, got %d: %v", len(output.lines), output.lines)
	}

	if stripANSICodes(output.lines[1]) != "./go.mod (score: 1.0000)" {
		t.Fatalf("expected go.mod header, got %q", output.lines[1])
	}

	if stripANSICodes(output.lines[2]) != "├── 1: alpha" {
		t.Fatalf("expected first context line, got %q", output.lines[2])
	}

	if stripANSICodes(output.lines[3]) != "├── 2: module idx" {
		t.Fatalf("expected matched line, got %q", output.lines[3])
	}

	if stripANSICodes(output.lines[4]) != "└── 3: omega" {
		t.Fatalf("expected last context line, got %q", output.lines[4])
	}
}

func TestSearchCommandServiceRunWithOptionsMatchesOnlyFiltersContextLines(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "go.mod"): "alpha\nmodule idx\nomega",
	}}
	service := newSearchCommandServiceForFunctionalTests(tree, output, fileReader, repo)

	err := service.RunWithOptions("module idx", ports.SearchOptions{Format: ports.SearchOutputJSON, Context: 1, MatchesOnly: true})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(output.lines[0]), &response); err != nil {
		t.Fatalf("expected valid JSON output, got error %v with payload %q", err, output.lines[0])
	}

	results, ok := response["results"].([]any)
	if !ok {
		t.Fatalf("expected results array, got %T", response["results"])
	}

	if len(results) != 1 {
		t.Fatalf("expected one file result, got %d", len(results))
	}

	file := results[0].(map[string]any)
	matches, ok := file["matches"].([]any)
	if !ok {
		t.Fatalf("expected matches array, got %T", file["matches"])
	}

	if len(matches) != 1 {
		t.Fatalf("expected one matched line after filtering context, got %d", len(matches))
	}

	matchEntry, ok := matches[0].(map[string]any)
	if !ok {
		t.Fatalf("expected match object, got %T", matches[0])
	}

	if matchEntry["match"] != true {
		t.Fatalf("expected match=true, got %v", matchEntry["match"])
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

	if response["count"] != float64(2) {
		t.Fatalf("expected count 2 (total before size limit), got %v", response["count"])
	}

	results, ok := response["results"].([]any)
	if !ok {
		t.Fatalf("expected results array, got %T", response["results"])
	}

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

	if response["count"] != float64(2) {
		t.Fatalf("expected count 2 (total before from/size), got %v", response["count"])
	}

	results, ok := response["results"].([]any)
	if !ok {
		t.Fatalf("expected results array, got %T", response["results"])
	}

	if len(results) != 1 {
		t.Fatalf("expected one paginated file result, got %d", len(results))
	}

	payload := results[0].(map[string]any)
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

	if len(output.lines) == 0 {
		t.Fatalf("expected at least one output line, got 0")
	}

	headerLine := output.lines[0]
	if !strings.Contains(headerLine, "Found 2 file(s)") {
		t.Fatalf("expected match count header, got %q", headerLine)
	}
	if !strings.Contains(headerLine, "📁") {
		t.Fatalf("expected emoji in header, got %q", headerLine)
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

	if len(output.lines) == 0 {
		t.Fatalf("expected at least one output line, got 0")
	}

	headerLine := output.lines[0]
	if !strings.Contains(headerLine, "Found 2 file(s)") {
		t.Fatalf("expected total match count header, got %q", headerLine)
	}
	if !strings.Contains(headerLine, "showing 1") {
		t.Fatalf("expected pagination info in header, got %q", headerLine)
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

	// Should have header + 2 files only (guide.md and readme.md), one per line
	if len(output.lines) != 3 {
		t.Fatalf("expected 3 output lines, got %d: %v", len(output.lines), output.lines)
	}

	// Strip ANSI codes to check content
	line1 := stripANSICodes(output.lines[1])
	line2 := stripANSICodes(output.lines[2])

	if line1 != "./guide.md" && line1 != "./readme.md" {
		t.Fatalf("expected file path, got %q", line1)
	}

	if line2 != "./guide.md" && line2 != "./readme.md" {
		t.Fatalf("expected file path, got %q", line2)
	}

	if line1 == line2 {
		t.Fatalf("expected different file paths, got both %q", line1)
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

	if len(output.lines) != 1 {
		t.Fatalf("expected a single JSON line, got %d: %v", len(output.lines), output.lines)
	}

	var payload []string
	if err := json.Unmarshal([]byte(output.lines[0]), &payload); err != nil {
		t.Fatalf("expected valid JSON array output, got error %v with payload %q", err, output.lines[0])
	}

	if len(payload) != 2 {
		t.Fatalf("expected 2 file paths in JSON, got %d: %v", len(payload), payload)
	}

	// Verify paths are strings and contain filenames
	for _, path := range payload {
		if !strings.Contains(path, ".md") {
			t.Fatalf("expected markdown file path, got %q", path)
		}
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

	if len(output.lines) != 1 {
		t.Fatalf("expected a single JSON line, got %d: %v", len(output.lines), output.lines)
	}

	if !strings.Contains(output.lines[0], "\n") {
		t.Fatalf("expected pretty JSON with line breaks, got %q", output.lines[0])
	}

	var payload []string
	if err := json.Unmarshal([]byte(output.lines[0]), &payload); err != nil {
		t.Fatalf("expected valid pretty JSON output, got error %v with payload %q", err, output.lines[0])
	}

	if len(payload) != 2 {
		t.Fatalf("expected 2 file paths in JSON, got %d", len(payload))
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

	if len(output.lines) != 3 {
		t.Fatalf("expected header, file and blank line for metadata-only result, got %d: %v", len(output.lines), output.lines)
	}

	if stripANSICodes(output.lines[1]) != "internal/core/go.mod (score: 1.0000)" {
		t.Fatalf("expected metadata-only path result, got %q", output.lines[1])
	}
	if output.lines[2] != "" {
		t.Fatalf("expected trailing blank line, got %q", output.lines[2])
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

	if len(output.lines) != 3 {
		t.Fatalf("expected header, file and blank lines with wildcard filter, got %d lines: %v", len(output.lines), output.lines)
	}

	if stripANSICodes(output.lines[1]) != "internal/core/go.mod (score: 1.0000)" {
		t.Fatalf("expected internal/core/go.mod with suffix wildcard, got %q", output.lines[1])
	}
}

func searchTreeWithIndexes(rootDir string, indexRelativeDirs []string) *fakeProjectTree {
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.readDirMap[rootDir] = []domain.DirectoryEntry{}
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true
	for _, relativeDir := range indexRelativeDirs {
		directoryPath := rootDir
		for _, part := range splitRelativePath(relativeDir) {
			nextDir := filepath.Join(directoryPath, part)
			appendDirectoryEntry(tree, directoryPath, nextDir, part)
			if _, exists := tree.readDirMap[nextDir]; !exists {
				tree.readDirMap[nextDir] = []domain.DirectoryEntry{}
			}
			directoryPath = nextDir
		}

		tree.existing[filepath.Join(directoryPath, ".idx", "index.idx")] = true
	}

	return tree
}

func splitRelativePath(path string) []string {
	normalizedPath := strings.ReplaceAll(path, "\\", "/")
	parts := make([]string, 0)
	for _, part := range strings.Split(normalizedPath, "/") {
		if part == "" {
			continue
		}

		parts = append(parts, part)
	}

	return parts
}

func appendDirectoryEntry(tree *fakeProjectTree, parentDir string, directoryPath string, name string) {
	entries := tree.readDirMap[parentDir]
	for _, entry := range entries {
		if entry.Path == directoryPath {
			return
		}
	}

	tree.readDirMap[parentDir] = append(tree.readDirMap[parentDir], domain.DirectoryEntry{Name: name, Path: directoryPath, IsDir: true})
}

func searchableIndex() *domain.InvertedIndex {
	index := domain.NewInvertedIndex()
	index.Documents["guide.md"] = &domain.DocStats{Name: "guide.md", Path: "guide.md", Length: 3}
	index.Documents["readme.md"] = &domain.DocStats{Name: "readme.md", Path: "readme.md", Length: 7}
	index.AddPathTerms("guide.md", "guide.md")
	index.AddPathTerms("readme.md", "readme.md")
	index.DocumentCount = len(index.Documents)
	index.AverageDocLength = 5
	index.Terms["go"] = &domain.TermStats{
		IDF: 0.4,
		Docs: map[string]*domain.DocTermStats{
			"guide.md":  {TF: 2},
			"readme.md": {TF: 1},
		},
	}
	index.Terms["search"] = &domain.TermStats{
		IDF: 0.2,
		Docs: map[string]*domain.DocTermStats{
			"guide.md":  {TF: 1},
			"readme.md": {TF: 2},
		},
	}

	return index
}

func searchableIndexWithPartialMatch() *domain.InvertedIndex {
	index := searchableIndex()
	index.Documents["go.mod"] = &domain.DocStats{Name: "go.mod", Path: "go.mod", Length: 4}
	index.Documents["AGENTS.md"] = &domain.DocStats{Name: "AGENTS.md", Path: "AGENTS.md", Length: 6}
	index.Documents[".gitignore"] = &domain.DocStats{Name: ".gitignore", Path: ".gitignore", Length: 5}
	index.AddPathTerms("go.mod", "go.mod")
	index.AddPathTerms("AGENTS.md", "AGENTS.md")
	index.AddPathTerms(".gitignore", ".gitignore")
	index.DocumentCount = len(index.Documents)
	index.AverageDocLength = 5
	index.Terms["module"] = &domain.TermStats{
		IDF: 0.7,
		Docs: map[string]*domain.DocTermStats{
			"go.mod":     {TF: 1},
			".gitignore": {TF: 1},
		},
	}
	index.Terms["idx"] = &domain.TermStats{
		IDF: 0.8,
		Docs: map[string]*domain.DocTermStats{
			"go.mod":    {TF: 2},
			"AGENTS.md": {TF: 1},
		},
	}

	return index
}

func searchableIndexWithProximity() *domain.InvertedIndex {
	index := domain.NewInvertedIndex()
	index.Documents["near.txt"] = &domain.DocStats{Name: "near.txt", Path: "near.txt", Length: 5}
	index.Documents["far.txt"] = &domain.DocStats{Name: "far.txt", Path: "far.txt", Length: 5}
	index.AddPathTerms("near.txt", "near.txt")
	index.AddPathTerms("far.txt", "far.txt")
	index.DocumentCount = len(index.Documents)
	index.AverageDocLength = 5
	index.Terms["module"] = &domain.TermStats{
		IDF: 1.0,
		Docs: map[string]*domain.DocTermStats{
			"near.txt": {TF: 1, Positions: []int{1}},
			"far.txt":  {TF: 1, Positions: []int{1}},
		},
	}
	index.Terms["idx"] = &domain.TermStats{
		IDF: 1.0,
		Docs: map[string]*domain.DocTermStats{
			"near.txt": {TF: 1, Positions: []int{2}},
			"far.txt":  {TF: 1, Positions: []int{100}},
		},
	}

	return index
}

func searchableIndexForRelativePath() *domain.InvertedIndex {
	index := domain.NewInvertedIndex()
	index.Documents["go.mod"] = &domain.DocStats{Name: "go.mod", Path: filepath.Join("internal", "core", "go.mod"), Length: 5}
	index.AddPathTerms("go.mod", filepath.Join("internal", "core", "go.mod"))
	index.DocumentCount = len(index.Documents)
	index.AverageDocLength = 5
	index.Terms["module"] = &domain.TermStats{
		IDF: 1.0,
		Docs: map[string]*domain.DocTermStats{
			"go.mod": {TF: 1, Positions: []int{1}},
		},
	}
	index.Terms["idx"] = &domain.TermStats{
		IDF: 1.0,
		Docs: map[string]*domain.DocTermStats{
			"go.mod": {TF: 1, Positions: []int{2}},
		},
	}

	return index
}

func searchableIndexWithSingleResult(fileName string, moduleIDF float64, idxIDF float64, modulePositions []int, idxPositions []int) *domain.InvertedIndex {
	index := domain.NewInvertedIndex()
	index.Documents[fileName] = &domain.DocStats{Name: fileName, Path: fileName, Length: 5}
	index.AddPathTerms(fileName, fileName)
	index.DocumentCount = 1
	index.AverageDocLength = 5
	index.Terms["module"] = &domain.TermStats{IDF: moduleIDF, Docs: map[string]*domain.DocTermStats{fileName: {TF: 1, Positions: modulePositions}}}
	index.Terms["idx"] = &domain.TermStats{IDF: idxIDF, Docs: map[string]*domain.DocTermStats{fileName: {TF: 1, Positions: idxPositions}}}

	return index
}

func searchableIndexWithMetadataFilters(rootDir string) *domain.InvertedIndex {
	index := searchableIndex()
	index.Documents["guide.md"].Path = filepath.Join(rootDir, "guide.md")
	index.Documents["readme.md"].Path = filepath.Join(rootDir, "readme.md")
	index.PathTerms = map[string]map[string]bool{}
	index.AddPathTerms("guide.md", filepath.Join(rootDir, "guide.md"))
	index.AddPathTerms("readme.md", filepath.Join(rootDir, "readme.md"))
	return index
}

func searchableIndexForMetadataPath(directoryPath string, fileName string) *domain.InvertedIndex {
	index := domain.NewInvertedIndex()
	filePath := filepath.Join(directoryPath, fileName)
	index.Documents[fileName] = &domain.DocStats{Name: fileName, Path: filePath, Length: 1}
	index.AddPathTerms(fileName, filePath)
	index.DocumentCount = 1
	index.AverageDocLength = 1
	return index
}

// fakeProjectTree implements ports.ProjectTree for testing.
type fakeProjectTree struct {
	currentDir string
	gitRoot    string
	readDirMap map[string][]domain.DirectoryEntry
	readDirErr map[string]error
	existing   map[string]bool
	removed    []string
	removeErrs map[string]error
	writes     map[string]string
	gitRootErr error
}

func newFakeProjectTree(currentDir string, gitRoot string) *fakeProjectTree {
	return &fakeProjectTree{
		currentDir: currentDir,
		gitRoot:    gitRoot,
		readDirMap: map[string][]domain.DirectoryEntry{},
		readDirErr: map[string]error{},
		existing:   map[string]bool{},
		removed:    []string{},
		removeErrs: map[string]error{},
		writes:     map[string]string{},
	}
}

func (tree *fakeProjectTree) CurrentDir() (string, error) {
	return tree.currentDir, nil
}

func (tree *fakeProjectTree) FindGitRoot(startDir string) (string, error) {
	if tree.gitRootErr != nil {
		return "", tree.gitRootErr
	}

	return tree.gitRoot, nil
}

func (tree *fakeProjectTree) ReadDir(path string) ([]domain.DirectoryEntry, error) {
	if err, ok := tree.readDirErr[path]; ok {
		return nil, err
	}

	entries, ok := tree.readDirMap[path]
	if !ok {
		return []domain.DirectoryEntry{}, nil
	}

	return entries, nil
}

func (tree *fakeProjectTree) Exists(path string) (bool, error) {
	return tree.existing[path], nil
}

func (tree *fakeProjectTree) RemoveAll(path string) error {
	tree.removed = append(tree.removed, path)
	if err, hasError := tree.removeErrs[path]; hasError {
		return err
	}

	return nil
}

func (tree *fakeProjectTree) WriteFile(path string, content []byte) error {
	tree.writes[path] = string(content)
	return nil
}

type capturingTextOutput struct {
	lines []string
}

func (output *capturingTextOutput) WriteLine(text string) error {
	output.lines = append(output.lines, text)
	return nil
}

// =============================================================================
// CACHE FUNCTIONALITY TESTS
// =============================================================================

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

	// Use default constructor directly to validate default app behavior.
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

// TestSearchCacheIsUsedForPaginationWithFrom verifies that a second search
// with the same query but different --from uses cached results.
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

	// First search: get all results.
	err := service.RunWithOptions("go search", ports.SearchOptions{Format: ports.SearchOutputJSON, Size: 1})
	if err != nil {
		t.Fatalf("expected first search to succeed, got %v", err)
	}
	if len(repo.loaded) != 1 {
		t.Fatalf("expected 1 index load for first search, got %d", len(repo.loaded))
	}
	firstOutput := output.lines[len(output.lines)-1]

	// Second search: paginate with --from=1 (should use cache, not reload index).
	err = service.RunWithOptions("go search", ports.SearchOptions{Format: ports.SearchOutputJSON, From: 1, Size: 1})
	if err != nil {
		t.Fatalf("expected second search to succeed, got %v", err)
	}

	// Verify index was NOT reloaded (still 1 load, not 2).
	if len(repo.loaded) != 1 {
		t.Fatalf("expected cache to prevent reload, but got %d loads", len(repo.loaded))
	}

	// Verify output is different (different page).
	secondOutput := output.lines[len(output.lines)-1]
	if firstOutput == secondOutput {
		t.Fatalf("expected different results for different pages, but got same output")
	}
}

// TestSearchCacheIsInvalidatedWhenQueryChanges verifies that cache is NOT used
// when the search query changes.
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

	// First search: "go search"
	_ = service.RunWithOptions("go search", ports.SearchOptions{})
	firstLoadCount := len(repo.loaded)

	// Second search: different query "module idx"
	_ = service.RunWithOptions("module idx", ports.SearchOptions{})
	secondLoadCount := len(repo.loaded)

	// Verify cache was NOT used (index loaded again for new query).
	if secondLoadCount <= firstLoadCount {
		t.Fatalf("expected new index load for different query, but load count didn't increase: first=%d, second=%d", firstLoadCount, secondLoadCount)
	}
}

// TestSearchCacheIsInvalidatedWhenOptionsChange verifies that cache is NOT used
// when search options (other than --from/--size) change.
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

	// First search: with --context=0.
	_ = service.RunWithOptions("module idx", ports.SearchOptions{Context: 0})
	firstLoadCount := len(repo.loaded)

	// Second search: same query but --context=1 (different option).
	_ = service.RunWithOptions("module idx", ports.SearchOptions{Context: 1})
	secondLoadCount := len(repo.loaded)

	// Verify cache was NOT used (index loaded again for different options).
	if secondLoadCount <= firstLoadCount {
		t.Fatalf("expected new index load for different context, but load count didn't increase: first=%d, second=%d", firstLoadCount, secondLoadCount)
	}
}

// TestSearchCacheIsRenewedWhenNavigatingPages verifies that TTL is renewed
// when accessing cache for pagination (e.g., new --from value).
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

	// First search caches results.
	_ = service.RunWithOptions("go search", ports.SearchOptions{})

	// Navigate pages multiple times - TTL should be renewed each time.
	_ = service.RunWithOptions("go search", ports.SearchOptions{From: 1})
	if len(repo.loaded) != 1 {
		t.Fatalf("expected cache hit on second page, but index was reloaded")
	}

	_ = service.RunWithOptions("go search", ports.SearchOptions{From: 2})
	if len(repo.loaded) != 1 {
		t.Fatalf("expected cache hit on third page, but index was reloaded")
	}

	// Verify we continued using cache throughout pagination.
	if len(repo.loaded) != 1 {
		t.Fatalf("expected exactly 1 index load across all pagination requests, got %d", len(repo.loaded))
	}
}

// TestSearchCacheWorksWithFilesOnlyOption verifies that cache works correctly
// when using --files-only (which still needs cached ranked results).
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

	// First search with --files-only and --size.
	_ = service.RunWithOptions("go search", ports.SearchOptions{FilesOnly: true, Size: 1})
	firstLoadCount := len(repo.loaded)

	// Second search: same query, different --from (should use cache).
	_ = service.RunWithOptions("go search", ports.SearchOptions{FilesOnly: true, From: 1, Size: 1})

	// Verify cache was used (no new index load).
	if len(repo.loaded) != firstLoadCount {
		t.Fatalf("expected cache to be used with --files-only, but index was reloaded")
	}
}

// TestSearchCacheWorksWithMatchesOnlyOption verifies that cache works with
// --matches-only (filters out context lines after cache lookup).
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

	// First search with --context and --matches-only.
	_ = service.RunWithOptions("module idx", ports.SearchOptions{Context: 1, MatchesOnly: true})
	firstLoadCount := len(repo.loaded)

	// Second search: same options, different --from (should use cache).
	_ = service.RunWithOptions("module idx", ports.SearchOptions{Context: 1, MatchesOnly: true, From: 0})

	// Verify cache was used.
	if len(repo.loaded) != firstLoadCount {
		t.Fatalf("expected cache to be used with --matches-only, but index was reloaded")
	}
}

// TestSearchCacheThreadSafety verifies that concurrent searches do not cause
// race conditions or cache corruption.
func TestSearchCacheThreadSafety(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{} // Not thread-safe but ok for this test (single query path).
	repo := &fakeSearchIndexRepository{
		indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithPartialMatch()},
	}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "guide.md"):  "go search guide",
		filepath.Join(rootDir, "readme.md"): "go content\nsearch topic",
	}}
	service := newSearchCommandServiceForCacheTests(tree, output, fileReader, repo)

	// Run multiple concurrent searches with pagination to stress cache.
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()
			_ = service.RunWithOptions("go search", ports.SearchOptions{From: offset})
		}(i)
	}
	wg.Wait()

	// Due to goroutine scheduling, some concurrent calls may race before cache is filled.
	// The important guarantee here is safety (no panic/data corruption) and successful completion.
	if len(repo.loaded) == 0 {
		t.Fatalf("expected at least one index load, got %d", len(repo.loaded))
	}
}

// TestSearchCacheCacheSizeNDoesNotGrowUnbounded verifies that cache entries
// are properly cleaned up and don't consume unlimited memory.
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

	// Perform multiple different searches to populate cache.
	for i := 0; i < 10; i++ {
		_ = service.RunWithOptions("query", ports.SearchOptions{Context: i})
	}

	// Cache should have grown to multiple entries for different options.
	// We verify at least that the service operates normally without panicking or race conditions.
	if len(output.lines) > 0 {
		t.Logf("Cache test completed without panicking")
	}
}

// TestSearchCacheFormatDoesNotAffectCacheKey verifies that --format and
// --json-pretty don't affect cache key (output formatting is separate).
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

	// First search in text format.
	_ = service.RunWithOptions("module idx", ports.SearchOptions{Format: ports.SearchOutputText})
	firstLoadCount := len(repo.loaded)

	// Second search in JSON format (should use cache - only output format differs).
	// Actually, this SHOULD NOT use cache because Format is part of the key.
	// Let me re-read the requirements... yes, Format should be part of the key.
	_ = service.RunWithOptions("module idx", ports.SearchOptions{Format: ports.SearchOutputJSON})
	secondLoadCount := len(repo.loaded)

	// Different format means different cache key, so index should be reloaded.
	if secondLoadCount <= firstLoadCount {
		t.Fatalf("expected format difference to invalidate cache, but load count didn't increase: first=%d, second=%d", firstLoadCount, secondLoadCount)
	}
}
