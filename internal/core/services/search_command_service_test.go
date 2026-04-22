package services_test

import (
	"errors"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"idx/internal/core/domain"
	"idx/internal/core/services"
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
	service := services.NewSearchCommandService(tree, output, fileReader, repo)

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
	service := services.NewSearchCommandService(tree, output, fileReader, repo)

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
	service := services.NewSearchCommandService(tree, output, fakeSearchFileReader{files: map[string]string{
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
	service := services.NewSearchCommandService(tree, &capturingTextOutput{}, fakeSearchFileReader{files: map[string]string{}}, repo)

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
	service := services.NewSearchCommandService(tree, output, fakeSearchFileReader{files: map[string]string{
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
	service := services.NewSearchCommandService(tree, output, fakeSearchFileReader{files: map[string]string{
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
	service := services.NewSearchCommandService(tree, output, fakeSearchFileReader{files: map[string]string{
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
	index.Documents["guide.md"] = &domain.DocStats{Length: 3}
	index.Documents["readme.md"] = &domain.DocStats{Length: 7}
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
	index.Documents["go.mod"] = &domain.DocStats{Length: 4}
	index.Documents["AGENTS.md"] = &domain.DocStats{Length: 6}
	index.Documents[".gitignore"] = &domain.DocStats{Length: 5}
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
	index.Documents["near.txt"] = &domain.DocStats{Length: 5}
	index.Documents["far.txt"] = &domain.DocStats{Length: 5}
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
	index.Documents["go.mod"] = &domain.DocStats{Length: 5}
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
	index.Documents[fileName] = &domain.DocStats{Length: 5}
	index.DocumentCount = 1
	index.AverageDocLength = 5
	index.Terms["module"] = &domain.TermStats{IDF: moduleIDF, Docs: map[string]*domain.DocTermStats{fileName: {TF: 1, Positions: modulePositions}}}
	index.Terms["idx"] = &domain.TermStats{IDF: idxIDF, Docs: map[string]*domain.DocTermStats{fileName: {TF: 1, Positions: idxPositions}}}

	return index
}
