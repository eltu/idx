package services_test

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"idx/internal/core/domain"
	"idx/internal/core/ports"
	"idx/internal/core/services"
)

type fakeProjectTree struct {
	currentDir string
	gitRoot    string
	readDirMap map[string][]domain.DirectoryEntry
	existing   map[string]bool
	removed    []string
	writes     map[string]string
	gitRootErr error
}

func newFakeProjectTree(currentDir string, gitRoot string) *fakeProjectTree {
	return &fakeProjectTree{
		currentDir: currentDir,
		gitRoot:    gitRoot,
		readDirMap: map[string][]domain.DirectoryEntry{},
		existing:   map[string]bool{},
		removed:    []string{},
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
	return nil
}

func (tree *fakeProjectTree) WriteFile(path string, content []byte) error {
	tree.writes[path] = string(content)
	return nil
}

type fakeIgnoreMatcherFactory struct {
	ignoredPaths map[string]bool
}

func (factory fakeIgnoreMatcherFactory) New(projectRoot string) (ports.IgnoreMatcher, error) {
	return fakeIgnoreMatcher(factory), nil
}

type fakeIgnoreMatcher struct {
	ignoredPaths map[string]bool
}

func (matcher fakeIgnoreMatcher) Matches(path string) (bool, error) {
	return matcher.ignoredPaths[path], nil
}

type fakeFileReader struct {
	files map[string]string
}

func (reader fakeFileReader) ReadFile(path string) (string, error) {
	content, ok := reader.files[path]
	if !ok {
		return "", errors.New("file not found")
	}
	return content, nil
}

type fakeBM25Indexer struct{}

func (indexer *fakeBM25Indexer) BuildIndex(documents map[string]string) (*domain.InvertedIndex, error) {
	index := domain.NewInvertedIndex()
	for docName := range documents {
		index.AddDocument(docName, 10)
	}
	index.DocumentCount = len(documents)
	index.CalculateAverageDocLen()
	return index, nil
}

type fakeIndexRepository struct {
	savedIndices map[string]*domain.InvertedIndex
}

func (repo *fakeIndexRepository) SaveIndex(directoryPath string, index *domain.InvertedIndex) error {
	repo.savedIndices[directoryPath] = index
	return nil
}

func (repo *fakeIndexRepository) LoadIndex(directoryPath string) (*domain.InvertedIndex, error) {
	index, ok := repo.savedIndices[directoryPath]
	if !ok {
		return nil, errors.New("index not found")
	}

	return index, nil
}

func TestInitCommandServiceRunWritesIndexFilesForAllowedEntries(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	childDir := filepath.Join(rootDir, "child")
	emptyDir := filepath.Join(rootDir, "empty")
	vendorDir := filepath.Join(rootDir, "vendor")

	tree := newFakeProjectTree(rootDir, rootDir)
	tree.readDirMap[rootDir] = []domain.DirectoryEntry{
		{Name: ".git", Path: filepath.Join(rootDir, ".git"), IsDir: true},
		{Name: ".idx", Path: filepath.Join(rootDir, ".idx"), IsDir: true},
		{Name: ".gitignore", Path: filepath.Join(rootDir, ".gitignore"), IsDir: false},
		{Name: "allowed.txt", Path: filepath.Join(rootDir, "allowed.txt"), IsDir: false},
		{Name: "child", Path: childDir, IsDir: true},
		{Name: "empty", Path: emptyDir, IsDir: true},
		{Name: "ignored.log", Path: filepath.Join(rootDir, "ignored.log"), IsDir: false},
		{Name: "vendor", Path: vendorDir, IsDir: true},
	}
	tree.readDirMap[childDir] = []domain.DirectoryEntry{{Name: "nested.txt", Path: filepath.Join(childDir, "nested.txt"), IsDir: false}}
	tree.readDirMap[emptyDir] = []domain.DirectoryEntry{}
	tree.readDirMap[vendorDir] = []domain.DirectoryEntry{{Name: "skip.txt", Path: filepath.Join(vendorDir, "skip.txt"), IsDir: false}}

	matcherFactory := fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{
		"ignored.log": true,
		"vendor/":     true,
	}}

	fileReader := fakeFileReader{files: map[string]string{
		filepath.Join(rootDir, ".gitignore"):  "*.log\nvendor/",
		filepath.Join(rootDir, "allowed.txt"): "hello world test",
		filepath.Join(childDir, "nested.txt"): "nested content here",
	}}

	indexer := &fakeBM25Indexer{}
	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*domain.InvertedIndex)}

	service := services.NewInitCommandService(tree, matcherFactory, &capturingTextOutput{}, fileReader, indexer, indexRepo)

	err := service.Run()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify root directory index was created
	if _, ok := indexRepo.savedIndices[rootDir]; !ok {
		t.Fatalf("expected index for root directory, got nothing")
	}

	// Verify root index has 2 files (not the ignored one)
	rootIndex := indexRepo.savedIndices[rootDir]
	if rootIndex.DocumentCount != 2 {
		t.Fatalf("expected root index to have 2 documents, got %d", rootIndex.DocumentCount)
	}

	// Verify child directory index was created
	if _, ok := indexRepo.savedIndices[childDir]; !ok {
		t.Fatalf("expected index for child directory, got nothing")
	}

	// Verify child index has 1 file
	childIndex := indexRepo.savedIndices[childDir]
	if childIndex.DocumentCount != 1 {
		t.Fatalf("expected child index to have 1 document, got %d", childIndex.DocumentCount)
	}

	// Verify empty directory index was created (even if empty)
	if _, ok := indexRepo.savedIndices[emptyDir]; !ok {
		t.Fatalf("expected index for empty directory, got nothing")
	}

	// Verify vendor directory index was NOT created (ignored)
	if _, ok := indexRepo.savedIndices[vendorDir]; ok {
		t.Fatalf("did not expect index for ignored directory %q", vendorDir)
	}
}

func TestInitCommandServiceRunRejectsDirectoryOutsideGitProject(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := newFakeProjectTree(rootDir, "")
	tree.gitRootErr = errors.New("directory \"/repo\" is not inside a git project: expected a path with a .git entry in the current directory or one of its parents")
	matcherFactory := fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}
	fileReader := fakeFileReader{files: make(map[string]string)}
	indexer := &fakeBM25Indexer{}
	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*domain.InvertedIndex)}

	service := services.NewInitCommandService(tree, matcherFactory, &capturingTextOutput{}, fileReader, indexer, indexRepo)

	err := service.Run()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestInitCommandServiceRunSkipsWhenIndexAlreadyExists(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true
	matcherFactory := fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}
	output := &capturingTextOutput{}
	fileReader := fakeFileReader{files: make(map[string]string)}
	indexer := &fakeBM25Indexer{}
	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*domain.InvertedIndex)}

	service := services.NewInitCommandService(tree, matcherFactory, output, fileReader, indexer, indexRepo)

	err := service.Run()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(output.lines) != 1 {
		t.Fatalf("expected 1 output line, got %d", len(output.lines))
	}

	if output.lines[0] != "ℹ️ Este projeto ja possui indice. Voce pode executar idx search." {
		t.Fatalf("unexpected output message %q", output.lines[0])
	}

	if len(indexRepo.savedIndices) != 0 {
		t.Fatalf("expected no index saves, got %d", len(indexRepo.savedIndices))
	}
}

func TestInitCommandServiceRunWithDebugWritesIndexPayload(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := newFakeProjectTree(rootDir, rootDir)
	matcherFactory := fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}
	output := &capturingTextOutput{}
	fileReader := fakeFileReader{files: map[string]string{
		filepath.Join(rootDir, "allowed.txt"): "hello world test",
	}}
	indexer := &fakeBM25Indexer{}
	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*domain.InvertedIndex)}

	tree.readDirMap[rootDir] = []domain.DirectoryEntry{
		{Name: "allowed.txt", Path: filepath.Join(rootDir, "allowed.txt"), IsDir: false},
	}

	service := services.NewInitCommandService(tree, matcherFactory, output, fileReader, indexer, indexRepo)

	err := service.RunWithDebug(true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(output.lines) != 2 {
		t.Fatalf("expected 2 output lines, got %d", len(output.lines))
	}

	if output.lines[0] != "✅ Index created. You can now run idx search." {
		t.Fatalf("unexpected output message %q", output.lines[0])
	}

	var payload domain.InvertedIndex
	if err := json.Unmarshal([]byte(output.lines[1]), &payload); err != nil {
		t.Fatalf("expected valid JSON debug payload, got %v", err)
	}

	if payload.DocumentCount != 1 {
		t.Fatalf("expected debug payload with 1 document, got %d", payload.DocumentCount)
	}
}

type capturingTextOutput struct {
	lines []string
}

func (output *capturingTextOutput) WriteLine(text string) error {
	output.lines = append(output.lines, text)
	return nil
}
