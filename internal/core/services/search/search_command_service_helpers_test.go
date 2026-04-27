package search_test

import (
	"errors"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

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
	index.Terms["module"] = &domain.TermStats{IDF: 1.0, Docs: map[string]*domain.DocTermStats{"go.mod": {TF: 1, Positions: []int{1}}}}
	index.Terms["idx"] = &domain.TermStats{IDF: 1.0, Docs: map[string]*domain.DocTermStats{"go.mod": {TF: 1, Positions: []int{2}}}}
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

func (tree *fakeProjectTree) CurrentDir() (string, error) { return tree.currentDir, nil }

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

func (tree *fakeProjectTree) Exists(path string) (bool, error) { return tree.existing[path], nil }

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

type capturingTextOutput struct{ lines []string }

func (output *capturingTextOutput) WriteLine(text string) error {
	output.lines = append(output.lines, text)
	return nil
}
