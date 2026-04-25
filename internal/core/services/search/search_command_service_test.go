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
	service := search.NewSearchCommandService(tree, output, fileReader, repo)

	err := service.Run("go search")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(repo.loaded) != 1 || repo.loaded[0] != rootDir {
		t.Fatalf("expected load for %q, got %v", rootDir, repo.loaded)
	}

	// guide.md (score 1.0000): 1 file header + 1 matching line + blank
	// readme.md (score 0.0000): 1 file header + 2 matching lines (go / search on separate lines) + blank
	if len(output.lines) != 7 {
		t.Fatalf("expected 7 output lines, got %d: %v", len(output.lines), output.lines)
	}

	if stripANSICodes(output.lines[0]) != "./guide.md (score: 1.0000)" {
		t.Fatalf("expected best result file header first, got %q", output.lines[0])
	}

	if stripANSICodes(output.lines[1]) != "└── 1: go search guide" {
		t.Fatalf("expected guide.md matched line, got %q", output.lines[1])
	}

	if stripANSICodes(output.lines[3]) != "./readme.md (score: 0.0000)" {
		t.Fatalf("expected second result file header, got %q", output.lines[3])
	}

	if stripANSICodes(output.lines[4]) != "├── 1: go content" {
		t.Fatalf("expected readme.md first matched line, got %q", output.lines[4])
	}

	if stripANSICodes(output.lines[5]) != "└── 2: search topic" {
		t.Fatalf("expected readme.md second matched line, got %q", output.lines[5])
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
	service := search.NewSearchCommandService(tree, output, fileReader, repo)

	err := service.Run("module idx")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// file header + 1 matched line + blank
	if len(output.lines) != 3 {
		t.Fatalf("expected 3 output lines, got %d: %v", len(output.lines), output.lines)
	}

	if stripANSICodes(output.lines[0]) != "./go.mod (score: 1.0000)" {
		t.Fatalf("expected only full match result, got %q", output.lines[0])
	}
}

func TestSearchCommandServiceRunWritesNoResultsMessage(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*domain.InvertedIndex{rootDir: searchableIndex()}}
	service := search.NewSearchCommandService(tree, output, fakeSearchFileReader{files: map[string]string{
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

	if output.lines[0] != "Nenhum resultado encontrado." {
		t.Fatalf("unexpected output message %q", output.lines[0])
	}
}

func TestSearchCommandServiceRunReturnsLoadError(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	repo := &fakeSearchIndexRepository{loadErr: errors.New("boom")}
	service := search.NewSearchCommandService(tree, &capturingTextOutput{}, fakeSearchFileReader{files: map[string]string{}}, repo)

	err := service.Run("go")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestSearchCommandServiceRunBoostsDocumentsWithNearbyTerms(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithProximity()}}
	service := search.NewSearchCommandService(tree, output, fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "near.txt"): "module idx",
		filepath.Join(rootDir, "far.txt"):  "module\nidx",
	}}, repo)

	err := service.Run("module idx")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// near.txt: header + 1 line + blank; far.txt: header + 2 lines + blank
	if len(output.lines) != 7 {
		t.Fatalf("expected 7 output lines, got %d: %v", len(output.lines), output.lines)
	}

	if stripANSICodes(output.lines[0]) != "./near.txt (score: 1.0000)" {
		t.Fatalf("expected nearby terms file first, got %q", output.lines[0])
	}

	if stripANSICodes(output.lines[1]) != "└── 1: module idx" {
		t.Fatalf("expected near.txt matched line, got %q", output.lines[1])
	}

	if stripANSICodes(output.lines[3]) != "./far.txt (score: 0.0000)" {
		t.Fatalf("expected far terms file second, got %q", output.lines[3])
	}

	if stripANSICodes(output.lines[4]) != "├── 1: module" {
		t.Fatalf("expected far.txt first line, got %q", output.lines[4])
	}

	if stripANSICodes(output.lines[5]) != "└── 2: idx" {
		t.Fatalf("expected far.txt second line, got %q", output.lines[5])
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
	service := search.NewSearchCommandService(tree, output, fakeSearchFileReader{files: map[string]string{
		filepath.Join(childDir, "go.mod"): "module idx",
	}}, repo)

	err := service.Run("module idx")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(output.lines) != 3 {
		t.Fatalf("expected 3 output lines, got %d: %v", len(output.lines), output.lines)
	}

	if stripANSICodes(output.lines[0]) != "internal/core/go.mod (score: 1.0000)" {
		t.Fatalf("expected project-relative path output, got %q", output.lines[0])
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
	service := search.NewSearchCommandService(tree, output, fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "root.md"):   "module idx",
		filepath.Join(childDir, "guide.md"): "module idx",
	}}, repo)

	err := service.Run("module idx")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// 2 files, each with header + 1 matched line + blank
	if len(output.lines) != 6 {
		t.Fatalf("expected 6 output lines, got %d: %v", len(output.lines), output.lines)
	}

	if stripANSICodes(output.lines[0]) != "docs/guide.md (score: 1.0000)" {
		t.Fatalf("expected child directory file header, got %q", output.lines[0])
	}

	if stripANSICodes(output.lines[3]) != "./root.md (score: 1.0000)" {
		t.Fatalf("expected root directory file header, got %q", output.lines[3])
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
	service := search.NewSearchCommandService(tree, output, fileReader, repo)

	err := service.RunWithOptions("module idx", ports.SearchOptions{Format: ports.SearchOutputJSON})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(output.lines) != 1 {
		t.Fatalf("expected a single JSON line, got %d: %v", len(output.lines), output.lines)
	}

	var payload []map[string]any
	if err := json.Unmarshal([]byte(output.lines[0]), &payload); err != nil {
		t.Fatalf("expected valid JSON output, got error %v with payload %q", err, output.lines[0])
	}

	if len(payload) != 1 {
		t.Fatalf("expected a single JSON result, got %d", len(payload))
	}

	if payload[0]["file"] != "./go.mod" {
		t.Fatalf("expected file ./go.mod, got %v", payload[0]["file"])
	}

	if payload[0]["name"] != "go.mod" {
		t.Fatalf("expected name go.mod, got %v", payload[0]["name"])
	}

	if payload[0]["path"] != "./go.mod" {
		t.Fatalf("expected path ./go.mod, got %v", payload[0]["path"])
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
	service := search.NewSearchCommandService(tree, output, fileReader, repo)

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

	var payload []map[string]any
	if err := json.Unmarshal([]byte(output.lines[0]), &payload); err != nil {
		t.Fatalf("expected valid pretty JSON output, got error %v with payload %q", err, output.lines[0])
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
	service := search.NewSearchCommandService(tree, output, fileReader, repo)

	err := service.RunWithOptions("module idx", ports.SearchOptions{Context: 1})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(output.lines) != 5 {
		t.Fatalf("expected 5 output lines, got %d: %v", len(output.lines), output.lines)
	}

	if stripANSICodes(output.lines[0]) != "./go.mod (score: 1.0000)" {
		t.Fatalf("expected go.mod header, got %q", output.lines[0])
	}

	if stripANSICodes(output.lines[1]) != "├── 1: alpha" {
		t.Fatalf("expected first context line, got %q", output.lines[1])
	}

	if stripANSICodes(output.lines[2]) != "├── 2: module idx" {
		t.Fatalf("expected matched line, got %q", output.lines[2])
	}

	if stripANSICodes(output.lines[3]) != "└── 3: omega" {
		t.Fatalf("expected last context line, got %q", output.lines[3])
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
	service := search.NewSearchCommandService(tree, output, fileReader, repo)

	err := service.RunWithOptions("module idx", ports.SearchOptions{Format: ports.SearchOutputJSON, Context: 1, MatchesOnly: true})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var payload []map[string]any
	if err := json.Unmarshal([]byte(output.lines[0]), &payload); err != nil {
		t.Fatalf("expected valid JSON output, got error %v with payload %q", err, output.lines[0])
	}

	if len(payload) != 1 {
		t.Fatalf("expected one file result, got %d", len(payload))
	}

	matches, ok := payload[0]["matches"].([]any)
	if !ok {
		t.Fatalf("expected matches array, got %T", payload[0]["matches"])
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

func TestSearchCommandServiceRunWithOptionsLimitRestrictsResultCount(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := searchTreeWithIndexes(rootDir, nil)
	output := &capturingTextOutput{}
	repo := &fakeSearchIndexRepository{indices: map[string]*domain.InvertedIndex{rootDir: searchableIndexWithPartialMatch()}}
	fileReader := fakeSearchFileReader{files: map[string]string{
		filepath.Join(rootDir, "guide.md"):  "go search guide",
		filepath.Join(rootDir, "readme.md"): "go content\nsearch topic",
	}}
	service := search.NewSearchCommandService(tree, output, fileReader, repo)

	err := service.RunWithOptions("go search", ports.SearchOptions{Format: ports.SearchOutputJSON, Limit: 1})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var payload []map[string]any
	if err := json.Unmarshal([]byte(output.lines[0]), &payload); err != nil {
		t.Fatalf("expected valid JSON output, got error %v with payload %q", err, output.lines[0])
	}

	if len(payload) != 1 {
		t.Fatalf("expected one file result with limit, got %d", len(payload))
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
	service := search.NewSearchCommandService(tree, output, fileReader, repo)

	err := service.RunWithOptions("go search", ports.SearchOptions{Format: ports.SearchOutputText, FilesOnly: true})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Should have 2 files only (guide.md and readme.md), one per line
	if len(output.lines) != 2 {
		t.Fatalf("expected 2 output lines, got %d: %v", len(output.lines), output.lines)
	}

	// Strip ANSI codes to check content
	line1 := stripANSICodes(output.lines[0])
	line2 := stripANSICodes(output.lines[1])

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
	service := search.NewSearchCommandService(tree, output, fileReader, repo)

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
	service := search.NewSearchCommandService(tree, output, fileReader, repo)

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
	service := search.NewSearchCommandService(tree, output, fakeSearchFileReader{files: map[string]string{}}, repo)

	err := service.RunWithOptions("", ports.SearchOptions{PathQuery: "internal core"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(output.lines) != 2 {
		t.Fatalf("expected header and blank line for metadata-only result, got %d: %v", len(output.lines), output.lines)
	}

	if stripANSICodes(output.lines[0]) != "internal/core/go.mod (score: 1.0000)" {
		t.Fatalf("expected metadata-only path result, got %q", output.lines[0])
	}
	if output.lines[1] != "" {
		t.Fatalf("expected trailing blank line, got %q", output.lines[1])
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
	service := search.NewSearchCommandService(tree, output, fakeSearchFileReader{files: map[string]string{}}, repo)

	err := service.RunWithOptions("", ports.SearchOptions{PathQuery: "*core"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(output.lines) != 2 {
		t.Fatalf("expected one metadata-only result, got %d lines: %v", len(output.lines), output.lines)
	}

	if stripANSICodes(output.lines[0]) != "internal/core/go.mod (score: 1.0000)" {
		t.Fatalf("expected internal/core/go.mod with suffix wildcard, got %q", output.lines[0])
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
