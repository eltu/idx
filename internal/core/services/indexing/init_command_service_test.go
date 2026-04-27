package indexing_test

import (
	"errors"
	"path/filepath"
	"testing"

	"idx/internal/core/domain"
	"idx/internal/core/services/indexing"
)

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

	matcherFactory := fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{"ignored.log": true, "vendor/": true}}
	fileReader := fakeFileReader{files: map[string]string{
		filepath.Join(rootDir, ".gitignore"):  "*.log\nvendor/",
		filepath.Join(rootDir, "allowed.txt"): "hello world test",
		filepath.Join(childDir, "nested.txt"): "nested content here",
	}}
	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*domain.InvertedIndex)}
	service := indexing.NewInitCommandService(tree, matcherFactory, &capturingTextOutput{}, fileReader, &fakeBM25Indexer{}, indexRepo, newFakeChecksumRepository())

	err := service.Run()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if indexRepo.savedIndices[rootDir].DocumentCount != 2 {
		t.Fatalf("expected root index to have 2 documents, got %d", indexRepo.savedIndices[rootDir].DocumentCount)
	}
	if indexRepo.savedIndices[childDir].DocumentCount != 1 {
		t.Fatalf("expected child index to have 1 document, got %d", indexRepo.savedIndices[childDir].DocumentCount)
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
	service := indexing.NewInitCommandService(tree, fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}, &capturingTextOutput{}, fakeFileReader{files: make(map[string]string)}, &fakeBM25Indexer{}, &fakeIndexRepository{savedIndices: make(map[string]*domain.InvertedIndex)}, newFakeChecksumRepository())

	err := service.Run()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestInitCommandServiceRunReturnsCurrentDirError(t *testing.T) {
	tree := newFakeProjectTree("", "")
	tree.currentErr = errors.New("cwd unavailable")

	service := indexing.NewInitCommandService(tree, fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}, &capturingTextOutput{}, fakeFileReader{files: map[string]string{}}, &fakeBM25Indexer{}, &fakeIndexRepository{savedIndices: map[string]*domain.InvertedIndex{}}, newFakeChecksumRepository())

	err := service.Run()
	if err == nil {
		t.Fatal("expected current directory error, got nil")
	}
}

func TestInitCommandServiceInspectReturnsCurrentDirError(t *testing.T) {
	tree := newFakeProjectTree("", "")
	tree.currentErr = errors.New("cwd unavailable")

	service := indexing.NewInitCommandService(tree, fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}, &capturingTextOutput{}, fakeFileReader{files: map[string]string{}}, &fakeBM25Indexer{}, &fakeIndexRepository{savedIndices: map[string]*domain.InvertedIndex{}}, newFakeChecksumRepository())

	err := service.Inspect(".")
	if err == nil {
		t.Fatal("expected current directory error, got nil")
	}
}

func TestInitCommandServiceRunReturnsErrorWhenDependenciesAreNil(t *testing.T) {
	service := indexing.NewInitCommandService(nil, nil, nil, nil, nil, nil, nil)

	err := service.Run()
	if err == nil {
		t.Fatal("expected dependency validation error, got nil")
	}
}

func TestInitCommandServiceInspectReturnsErrorWhenDependenciesAreNil(t *testing.T) {
	service := indexing.NewInitCommandService(nil, nil, nil, nil, nil, nil, nil)

	err := service.Inspect(".")
	if err == nil {
		t.Fatal("expected dependency validation error, got nil")
	}
}

func TestInitCommandServiceRunReturnsMatcherFactoryError(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := newFakeProjectTree(rootDir, rootDir)
	service := indexing.NewInitCommandService(tree, fakeIgnoreMatcherFactory{err: errors.New("bad gitignore")}, &capturingTextOutput{}, fakeFileReader{files: map[string]string{}}, &fakeBM25Indexer{}, &fakeIndexRepository{savedIndices: map[string]*domain.InvertedIndex{}}, newFakeChecksumRepository())

	err := service.Run()
	if err == nil {
		t.Fatal("expected matcher factory error, got nil")
	}
}

func TestInitCommandServiceRunPropagatesChildDirectoryReadError(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	childDir := filepath.Join(rootDir, "child")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.readDirMap[rootDir] = []domain.DirectoryEntry{{Name: "child", Path: childDir, IsDir: true}}
	tree.readDirErr[childDir] = errors.New("cannot read child")

	service := indexing.NewInitCommandService(tree, fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}, &capturingTextOutput{}, fakeFileReader{files: map[string]string{}}, &fakeBM25Indexer{}, &fakeIndexRepository{savedIndices: map[string]*domain.InvertedIndex{}}, newFakeChecksumRepository())

	err := service.Run()
	if err == nil {
		t.Fatal("expected child directory read error, got nil")
	}
}

func TestInitCommandServiceRunPropagatesFileReadErrorOnIndexBuild(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.readDirMap[rootDir] = []domain.DirectoryEntry{{Name: "missing.txt", Path: filepath.Join(rootDir, "missing.txt"), IsDir: false}}

	service := indexing.NewInitCommandService(tree, fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}, &capturingTextOutput{}, fakeFileReader{files: map[string]string{}}, &fakeBM25Indexer{}, &fakeIndexRepository{savedIndices: map[string]*domain.InvertedIndex{}}, newFakeChecksumRepository())

	err := service.Run()
	if err == nil {
		t.Fatal("expected file read error, got nil")
	}
}

func TestInitCommandServiceRunSkipsWhenIndexAlreadyExists(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true
	output := &capturingTextOutput{}
	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*domain.InvertedIndex)}
	service := indexing.NewInitCommandService(tree, fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}, output, fakeFileReader{files: make(map[string]string)}, &fakeBM25Indexer{}, indexRepo, newFakeChecksumRepository())

	err := service.Run()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if output.lines[0] != "ℹ️ This project is already indexed. You can run idx search." {
		t.Fatalf("unexpected output message %q", output.lines[0])
	}
}
