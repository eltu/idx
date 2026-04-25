package services_test

import (
	"crypto/sha256"
	"encoding/hex"
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

func (indexer *fakeBM25Indexer) BuildIndex(documents []domain.IndexDocument) (*domain.InvertedIndex, error) {
	index := domain.NewInvertedIndex()
	for _, document := range documents {
		index.AddDocument(document.Name, document.Path, 10)
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

type fakeChecksumRepository struct {
	checksums map[string]map[string]string
	exists    map[string]bool
	saveCount map[string]int
}

func newFakeChecksumRepository() *fakeChecksumRepository {
	return &fakeChecksumRepository{
		checksums: map[string]map[string]string{},
		exists:    map[string]bool{},
		saveCount: map[string]int{},
	}
}

func (repo *fakeChecksumRepository) Load(directoryPath string) (map[string]string, bool, error) {
	if !repo.exists[directoryPath] {
		return map[string]string{}, false, nil
	}

	current := repo.checksums[directoryPath]
	cloned := make(map[string]string, len(current))
	for fileName, checksum := range current {
		cloned[fileName] = checksum
	}

	return cloned, true, nil
}

func (repo *fakeChecksumRepository) Save(directoryPath string, checksums map[string]string) error {
	cloned := make(map[string]string, len(checksums))
	for fileName, checksum := range checksums {
		cloned[fileName] = checksum
	}

	repo.exists[directoryPath] = true
	repo.checksums[directoryPath] = cloned
	repo.saveCount[directoryPath]++
	return nil
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
	checksumRepo := newFakeChecksumRepository()
	service := services.NewInitCommandService(tree, matcherFactory, &capturingTextOutput{}, fileReader, indexer, indexRepo, checksumRepo)

	err := service.Run()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if _, ok := indexRepo.savedIndices[rootDir]; !ok {
		t.Fatalf("expected index for root directory, got nothing")
	}

	rootIndex := indexRepo.savedIndices[rootDir]
	if rootIndex.DocumentCount != 2 {
		t.Fatalf("expected root index to have 2 documents, got %d", rootIndex.DocumentCount)
	}

	if _, ok := indexRepo.savedIndices[childDir]; !ok {
		t.Fatalf("expected index for child directory, got nothing")
	}

	childIndex := indexRepo.savedIndices[childDir]
	if childIndex.DocumentCount != 1 {
		t.Fatalf("expected child index to have 1 document, got %d", childIndex.DocumentCount)
	}

	if _, ok := indexRepo.savedIndices[emptyDir]; ok {
		t.Fatalf("did not expect index for empty directory %q", emptyDir)
	}

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
	checksumRepo := newFakeChecksumRepository()
	service := services.NewInitCommandService(tree, matcherFactory, &capturingTextOutput{}, fileReader, indexer, indexRepo, checksumRepo)

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
	checksumRepo := newFakeChecksumRepository()
	service := services.NewInitCommandService(tree, matcherFactory, output, fileReader, indexer, indexRepo, checksumRepo)

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

func TestInitCommandServiceSyncRebuildsAllIndexedDirectories(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	childDir := filepath.Join(rootDir, "child")
	otherDir := filepath.Join(rootDir, "other")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true
	tree.existing[filepath.Join(childDir, ".idx", "index.idx")] = true
	tree.readDirMap[rootDir] = []domain.DirectoryEntry{
		{Name: ".idx", Path: filepath.Join(rootDir, ".idx"), IsDir: true},
		{Name: "root.txt", Path: filepath.Join(rootDir, "root.txt"), IsDir: false},
		{Name: "child", Path: childDir, IsDir: true},
		{Name: "other", Path: otherDir, IsDir: true},
	}
	tree.readDirMap[childDir] = []domain.DirectoryEntry{{Name: "nested.txt", Path: filepath.Join(childDir, "nested.txt"), IsDir: false}}
	tree.readDirMap[otherDir] = []domain.DirectoryEntry{{Name: "other.txt", Path: filepath.Join(otherDir, "other.txt"), IsDir: false}}
	matcherFactory := fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}
	output := &capturingTextOutput{}
	fileReader := fakeFileReader{files: map[string]string{
		filepath.Join(rootDir, "root.txt"):    "root content",
		filepath.Join(childDir, "nested.txt"): "nested content",
		filepath.Join(otherDir, "other.txt"):  "other content",
	}}
	indexer := &fakeBM25Indexer{}
	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*domain.InvertedIndex)}
	checksumRepo := newFakeChecksumRepository()
	service := services.NewInitCommandService(tree, matcherFactory, output, fileReader, indexer, indexRepo, checksumRepo)

	err := service.Sync()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if indexRepo.savedIndices[rootDir].DocumentCount != 1 {
		t.Fatalf("expected synced root index with 1 document, got %d", indexRepo.savedIndices[rootDir].DocumentCount)
	}

	if indexRepo.savedIndices[childDir].DocumentCount != 1 {
		t.Fatalf("expected synced child index with 1 document, got %d", indexRepo.savedIndices[childDir].DocumentCount)
	}

	if indexRepo.savedIndices[otherDir].DocumentCount != 1 {
		t.Fatalf("expected sync to create a new index for %q with 1 document, got %d", otherDir, indexRepo.savedIndices[otherDir].DocumentCount)
	}

	if len(output.lines) != 1 || output.lines[0] != "✅ Project indices synchronized." {
		t.Fatalf("unexpected sync output %v", output.lines)
	}
}

func TestInitCommandServiceSyncSkipsReindexWhenChecksumsUnchanged(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	filePath := filepath.Join(rootDir, "root.txt")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true
	tree.readDirMap[rootDir] = []domain.DirectoryEntry{
		{Name: ".idx", Path: filepath.Join(rootDir, ".idx"), IsDir: true},
		{Name: "root.txt", Path: filePath, IsDir: false},
	}

	matcherFactory := fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}
	output := &capturingTextOutput{}
	fileReader := fakeFileReader{files: map[string]string{filePath: "root content"}}
	indexer := &fakeBM25Indexer{}
	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*domain.InvertedIndex)}
	checksumRepo := newFakeChecksumRepository()
	checksumRepo.exists[rootDir] = true
	checksumRepo.checksums[rootDir] = map[string]string{"root.txt": checksumFromContent("root content")}
	service := services.NewInitCommandService(tree, matcherFactory, output, fileReader, indexer, indexRepo, checksumRepo)

	err := service.Sync()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(indexRepo.savedIndices) != 0 {
		t.Fatalf("expected no reindex saves for unchanged checksum, got %d", len(indexRepo.savedIndices))
	}
}

func TestInitCommandServiceSyncReindexesWhenChecksumsChanged(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	filePath := filepath.Join(rootDir, "root.txt")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true
	tree.readDirMap[rootDir] = []domain.DirectoryEntry{
		{Name: ".idx", Path: filepath.Join(rootDir, ".idx"), IsDir: true},
		{Name: "root.txt", Path: filePath, IsDir: false},
	}

	matcherFactory := fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}
	output := &capturingTextOutput{}
	fileReader := fakeFileReader{files: map[string]string{filePath: "new root content"}}
	indexer := &fakeBM25Indexer{}
	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*domain.InvertedIndex)}
	checksumRepo := newFakeChecksumRepository()
	checksumRepo.exists[rootDir] = true
	checksumRepo.checksums[rootDir] = map[string]string{"root.txt": checksumFromContent("old root content")}
	service := services.NewInitCommandService(tree, matcherFactory, output, fileReader, indexer, indexRepo, checksumRepo)

	err := service.Sync()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if indexRepo.savedIndices[rootDir].DocumentCount != 1 {
		t.Fatalf("expected synced root index with 1 document, got %d", indexRepo.savedIndices[rootDir].DocumentCount)
	}

	if checksumRepo.saveCount[rootDir] != 1 {
		t.Fatalf("expected checksum update count 1, got %d", checksumRepo.saveCount[rootDir])
	}
}

func TestInitCommandServiceSyncRemovesIndexWhenDirectoryHasNoIndexableFiles(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true
	tree.readDirMap[rootDir] = []domain.DirectoryEntry{
		{Name: ".idx", Path: filepath.Join(rootDir, ".idx"), IsDir: true},
	}

	matcherFactory := fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}
	output := &capturingTextOutput{}
	fileReader := fakeFileReader{files: map[string]string{}}
	indexer := &fakeBM25Indexer{}
	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*domain.InvertedIndex)}
	checksumRepo := newFakeChecksumRepository()
	service := services.NewInitCommandService(tree, matcherFactory, output, fileReader, indexer, indexRepo, checksumRepo)

	err := service.Sync()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	removedIndexPath := filepath.Join(rootDir, ".idx")
	if len(tree.removed) != 1 || tree.removed[0] != removedIndexPath {
		t.Fatalf("expected removed index path %q, got %v", removedIndexPath, tree.removed)
	}
}

func TestInitCommandServiceSyncRemovesIndexForNowIgnoredDirectory(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	ignoredDir := filepath.Join(rootDir, "vendor")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true
	tree.existing[filepath.Join(ignoredDir, ".idx", "index.idx")] = true
	tree.readDirMap[rootDir] = []domain.DirectoryEntry{
		{Name: ".idx", Path: filepath.Join(rootDir, ".idx"), IsDir: true},
		{Name: "root.txt", Path: filepath.Join(rootDir, "root.txt"), IsDir: false},
		{Name: "vendor", Path: ignoredDir, IsDir: true},
	}
	tree.readDirMap[ignoredDir] = []domain.DirectoryEntry{{Name: "dep.txt", Path: filepath.Join(ignoredDir, "dep.txt"), IsDir: false}}

	matcherFactory := fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{"vendor/": true}}
	output := &capturingTextOutput{}
	fileReader := fakeFileReader{files: map[string]string{filepath.Join(rootDir, "root.txt"): "root content"}}
	indexer := &fakeBM25Indexer{}
	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*domain.InvertedIndex)}
	checksumRepo := newFakeChecksumRepository()
	service := services.NewInitCommandService(tree, matcherFactory, output, fileReader, indexer, indexRepo, checksumRepo)

	err := service.Sync()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	removedIndexPath := filepath.Join(ignoredDir, ".idx")
	if len(tree.removed) != 1 || tree.removed[0] != removedIndexPath {
		t.Fatalf("expected removed index path %q, got %v", removedIndexPath, tree.removed)
	}

	if _, exists := indexRepo.savedIndices[ignoredDir]; exists {
		t.Fatalf("did not expect ignored directory %q to be reindexed", ignoredDir)
	}
}

func TestInitCommandServiceSyncCreatesIndexWhenDirectoryLeavesGitignore(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	newlyAllowedDir := filepath.Join(rootDir, "src")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true
	tree.readDirMap[rootDir] = []domain.DirectoryEntry{
		{Name: ".idx", Path: filepath.Join(rootDir, ".idx"), IsDir: true},
		{Name: "root.txt", Path: filepath.Join(rootDir, "root.txt"), IsDir: false},
		{Name: "src", Path: newlyAllowedDir, IsDir: true},
	}
	tree.readDirMap[newlyAllowedDir] = []domain.DirectoryEntry{{Name: "app.txt", Path: filepath.Join(newlyAllowedDir, "app.txt"), IsDir: false}}

	matcherFactory := fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}
	output := &capturingTextOutput{}
	fileReader := fakeFileReader{files: map[string]string{
		filepath.Join(rootDir, "root.txt"):        "root content",
		filepath.Join(newlyAllowedDir, "app.txt"): "source content",
	}}
	indexer := &fakeBM25Indexer{}
	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*domain.InvertedIndex)}
	checksumRepo := newFakeChecksumRepository()
	service := services.NewInitCommandService(tree, matcherFactory, output, fileReader, indexer, indexRepo, checksumRepo)

	err := service.Sync()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if indexRepo.savedIndices[newlyAllowedDir].DocumentCount != 1 {
		t.Fatalf("expected newly allowed directory %q to be indexed with 1 document, got %d", newlyAllowedDir, indexRepo.savedIndices[newlyAllowedDir].DocumentCount)
	}
}

func TestInitCommandServiceSyncReindexesDirectoryWhenFileLeavesGitignore(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true
	tree.readDirMap[rootDir] = []domain.DirectoryEntry{
		{Name: ".idx", Path: filepath.Join(rootDir, ".idx"), IsDir: true},
		{Name: "allowed.txt", Path: filepath.Join(rootDir, "allowed.txt"), IsDir: false},
		{Name: "debug.log", Path: filepath.Join(rootDir, "debug.log"), IsDir: false},
	}

	matcherFactory := fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}
	output := &capturingTextOutput{}
	fileReader := fakeFileReader{files: map[string]string{
		filepath.Join(rootDir, "allowed.txt"): "allowed content",
		filepath.Join(rootDir, "debug.log"):   "log content",
	}}
	indexer := &fakeBM25Indexer{}
	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*domain.InvertedIndex)}
	checksumRepo := newFakeChecksumRepository()
	service := services.NewInitCommandService(tree, matcherFactory, output, fileReader, indexer, indexRepo, checksumRepo)

	err := service.Sync()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if indexRepo.savedIndices[rootDir].DocumentCount != 2 {
		t.Fatalf("expected root index to include file that left .gitignore with 2 documents, got %d", indexRepo.savedIndices[rootDir].DocumentCount)
	}
}

func TestInitCommandServiceSyncRequiresProjectRootIndex(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := newFakeProjectTree(rootDir, rootDir)
	matcherFactory := fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}
	output := &capturingTextOutput{}
	fileReader := fakeFileReader{files: make(map[string]string)}
	indexer := &fakeBM25Indexer{}
	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*domain.InvertedIndex)}
	checksumRepo := newFakeChecksumRepository()
	service := services.NewInitCommandService(tree, matcherFactory, output, fileReader, indexer, indexRepo, checksumRepo)

	err := service.Sync()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if len(indexRepo.savedIndices) != 0 {
		t.Fatalf("expected no synced indices, got %d", len(indexRepo.savedIndices))
	}
}

func TestInitCommandServiceSyncMustRunFromProjectRoot(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	childDir := filepath.Join(rootDir, "child")
	tree := newFakeProjectTree(childDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true
	matcherFactory := fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}
	output := &capturingTextOutput{}
	fileReader := fakeFileReader{files: make(map[string]string)}
	indexer := &fakeBM25Indexer{}
	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*domain.InvertedIndex)}
	checksumRepo := newFakeChecksumRepository()
	service := services.NewInitCommandService(tree, matcherFactory, output, fileReader, indexer, indexRepo, checksumRepo)

	err := service.Sync()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if len(indexRepo.savedIndices) != 0 {
		t.Fatalf("expected no synced indices, got %d", len(indexRepo.savedIndices))
	}
}

func TestInitCommandServiceInspectWritesIndexPayloadFromPath(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true
	matcherFactory := fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}
	output := &capturingTextOutput{}
	fileReader := fakeFileReader{files: map[string]string{
		filepath.Join(rootDir, "allowed.txt"): "hello world test",
	}}
	indexer := &fakeBM25Indexer{}
	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*domain.InvertedIndex)}
	checksumRepo := newFakeChecksumRepository()

	tree.readDirMap[rootDir] = []domain.DirectoryEntry{
		{Name: "allowed.txt", Path: filepath.Join(rootDir, "allowed.txt"), IsDir: false},
	}

	preBuiltIndex := domain.NewInvertedIndex()
	preBuiltIndex.DocumentCount = 1
	indexRepo.savedIndices[rootDir] = preBuiltIndex

	service := services.NewInitCommandService(tree, matcherFactory, output, fileReader, indexer, indexRepo, checksumRepo)

	err := service.Inspect(".")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(output.lines) != 1 {
		t.Fatalf("expected 1 output line, got %d", len(output.lines))
	}

	var payload domain.InvertedIndex
	if err := json.Unmarshal([]byte(output.lines[0]), &payload); err != nil {
		t.Fatalf("expected valid JSON inspect payload, got %v", err)
	}

	if payload.DocumentCount != 1 {
		t.Fatalf("expected inspect payload with 1 document, got %d", payload.DocumentCount)
	}
}

func TestInitCommandServiceInspectFailsWhenNoIndexExists(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := newFakeProjectTree(rootDir, rootDir)
	matcherFactory := fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}
	output := &capturingTextOutput{}
	fileReader := fakeFileReader{files: make(map[string]string)}
	indexer := &fakeBM25Indexer{}
	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*domain.InvertedIndex)}
	checksumRepo := newFakeChecksumRepository()
	service := services.NewInitCommandService(tree, matcherFactory, output, fileReader, indexer, indexRepo, checksumRepo)

	err := service.Inspect("internal")
	if err == nil {
		t.Fatal("expected an error when no index exists, got nil")
	}

	if len(indexRepo.savedIndices) != 0 {
		t.Fatalf("expected no index saves (inspect is read-only), got %d", len(indexRepo.savedIndices))
	}

	if len(output.lines) != 0 {
		t.Fatalf("expected no output lines on failure, got %d", len(output.lines))
	}
}

func TestInitCommandServiceInspectLoadsNestedIndexFromRelativePath(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	nestedDir := filepath.Join(rootDir, "internal")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(nestedDir, ".idx", "index.idx")] = true
	matcherFactory := fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}
	output := &capturingTextOutput{}
	fileReader := fakeFileReader{files: make(map[string]string)}
	indexer := &fakeBM25Indexer{}
	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*domain.InvertedIndex)}
	checksumRepo := newFakeChecksumRepository()

	preBuiltIndex := domain.NewInvertedIndex()
	preBuiltIndex.DocumentCount = 2
	indexRepo.savedIndices[nestedDir] = preBuiltIndex

	service := services.NewInitCommandService(tree, matcherFactory, output, fileReader, indexer, indexRepo, checksumRepo)

	err := service.Inspect("internal/")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(output.lines) != 1 {
		t.Fatalf("expected 1 output line, got %d", len(output.lines))
	}

	var payload domain.InvertedIndex
	if err := json.Unmarshal([]byte(output.lines[0]), &payload); err != nil {
		t.Fatalf("expected valid JSON inspect payload, got %v", err)
	}

	if payload.DocumentCount != 2 {
		t.Fatalf("expected inspect payload with 2 documents, got %d", payload.DocumentCount)
	}
}

func checksumFromContent(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

type capturingTextOutput struct {
	lines []string
}

func (output *capturingTextOutput) WriteLine(text string) error {
	output.lines = append(output.lines, text)
	return nil
}
