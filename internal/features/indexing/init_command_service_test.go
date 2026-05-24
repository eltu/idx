package indexing_test

import (
	"errors"
	"path/filepath"
	"testing"

	"idx/internal/features/indexing"
	"idx/internal/shared/filesystem"
)

func TestInitCommandServiceRunWritesIndexFilesForAllowedEntries(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	childDir := filepath.Join(rootDir, "child")
	emptyDir := filepath.Join(rootDir, "empty")
	vendorDir := filepath.Join(rootDir, "vendor")

	tree := newFakeProjectTree(rootDir, rootDir)
	tree.readDirMap[rootDir] = []filesystem.DirectoryEntry{
		{Name: ".git", Path: filepath.Join(rootDir, ".git"), IsDir: true},
		{Name: ".idx", Path: filepath.Join(rootDir, ".idx"), IsDir: true},
		{Name: ".gitignore", Path: filepath.Join(rootDir, ".gitignore"), IsDir: false},
		{Name: "allowed.txt", Path: filepath.Join(rootDir, "allowed.txt"), IsDir: false},
		{Name: "child", Path: childDir, IsDir: true},
		{Name: "empty", Path: emptyDir, IsDir: true},
		{Name: "ignored.log", Path: filepath.Join(rootDir, "ignored.log"), IsDir: false},
		{Name: "vendor", Path: vendorDir, IsDir: true},
	}
	tree.readDirMap[childDir] = []filesystem.DirectoryEntry{{Name: "nested.txt", Path: filepath.Join(childDir, "nested.txt"), IsDir: false}}
	tree.readDirMap[emptyDir] = []filesystem.DirectoryEntry{}
	tree.readDirMap[vendorDir] = []filesystem.DirectoryEntry{{Name: "skip.txt", Path: filepath.Join(vendorDir, "skip.txt"), IsDir: false}}

	matcherFactory := fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{"ignored.log": true, "vendor/": true}}
	fileReader := fakeFileReader{files: map[string]string{
		filepath.Join(rootDir, ".gitignore"):  "*.log\nvendor/",
		filepath.Join(rootDir, "allowed.txt"): "hello world test",
		filepath.Join(childDir, "nested.txt"): "nested content here",
	}}
	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*indexing.InvertedIndex)}
	service := indexing.NewInitCommandService(indexing.InitCommandServiceDeps{
		ProjectTree:    tree,
		MatcherFactory: matcherFactory,
		Output:         &capturingTextOutput{},
		FileReader:     fileReader,
		Indexer:        &fakeBM25Indexer{},
		IndexRepo:      indexRepo,
		ChecksumRepo:   newFakeChecksumRepository(),
		DaemonRepo:     nil,
	})

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
	service := indexing.NewInitCommandService(indexing.InitCommandServiceDeps{
		ProjectTree:    tree,
		MatcherFactory: fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}},
		Output:         &capturingTextOutput{},
		FileReader:     fakeFileReader{files: make(map[string]string)},
		Indexer:        &fakeBM25Indexer{},
		IndexRepo:      &fakeIndexRepository{savedIndices: make(map[string]*indexing.InvertedIndex)},
		ChecksumRepo:   newFakeChecksumRepository(),
		DaemonRepo:     nil,
	})

	err := service.Run()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestInitCommandServiceRunReturnsCurrentDirError(t *testing.T) {
	tree := newFakeProjectTree("", "")
	tree.currentErr = errors.New("cwd unavailable")

	service := indexing.NewInitCommandService(indexing.InitCommandServiceDeps{
		ProjectTree:    tree,
		MatcherFactory: fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}},
		Output:         &capturingTextOutput{},
		FileReader:     fakeFileReader{files: map[string]string{}},
		Indexer:        &fakeBM25Indexer{},
		IndexRepo:      &fakeIndexRepository{savedIndices: map[string]*indexing.InvertedIndex{}},
		ChecksumRepo:   newFakeChecksumRepository(),
		DaemonRepo:     nil,
	})

	err := service.Run()
	if err == nil {
		t.Fatal("expected current directory error, got nil")
	}
}

func TestInitCommandServiceInspectReturnsCurrentDirError(t *testing.T) {
	tree := newFakeProjectTree("", "")
	tree.currentErr = errors.New("cwd unavailable")

	service := indexing.NewInitCommandService(indexing.InitCommandServiceDeps{
		ProjectTree:    tree,
		MatcherFactory: fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}},
		Output:         &capturingTextOutput{},
		FileReader:     fakeFileReader{files: map[string]string{}},
		Indexer:        &fakeBM25Indexer{},
		IndexRepo:      &fakeIndexRepository{savedIndices: map[string]*indexing.InvertedIndex{}},
		ChecksumRepo:   newFakeChecksumRepository(),
		DaemonRepo:     nil,
	})

	err := service.Inspect(".")
	if err == nil {
		t.Fatal("expected current directory error, got nil")
	}
}

func TestInitCommandServiceRunReturnsErrorWhenDependenciesAreNil(t *testing.T) {
	service := indexing.NewInitCommandService(indexing.InitCommandServiceDeps{})

	err := service.Run()
	if err == nil {
		t.Fatal("expected dependency validation error, got nil")
	}
}

func TestInitCommandServiceInspectReturnsErrorWhenDependenciesAreNil(t *testing.T) {
	service := indexing.NewInitCommandService(indexing.InitCommandServiceDeps{})

	err := service.Inspect(".")
	if err == nil {
		t.Fatal("expected dependency validation error, got nil")
	}
}

func TestInitCommandServiceRunReturnsMatcherFactoryError(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := newFakeProjectTree(rootDir, rootDir)
	service := indexing.NewInitCommandService(indexing.InitCommandServiceDeps{
		ProjectTree:    tree,
		MatcherFactory: fakeIgnoreMatcherFactory{err: errors.New("bad gitignore")},
		Output:         &capturingTextOutput{},
		FileReader:     fakeFileReader{files: map[string]string{}},
		Indexer:        &fakeBM25Indexer{},
		IndexRepo:      &fakeIndexRepository{savedIndices: map[string]*indexing.InvertedIndex{}},
		ChecksumRepo:   newFakeChecksumRepository(),
		DaemonRepo:     nil,
	})

	err := service.Run()
	if err == nil {
		t.Fatal("expected matcher factory error, got nil")
	}
}

func TestInitCommandServiceRunPropagatesChildDirectoryReadError(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	childDir := filepath.Join(rootDir, "child")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.readDirMap[rootDir] = []filesystem.DirectoryEntry{{Name: "child", Path: childDir, IsDir: true}}
	tree.readDirErr[childDir] = errors.New("cannot read child")

	service := indexing.NewInitCommandService(indexing.InitCommandServiceDeps{
		ProjectTree:    tree,
		MatcherFactory: fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}},
		Output:         &capturingTextOutput{},
		FileReader:     fakeFileReader{files: map[string]string{}},
		Indexer:        &fakeBM25Indexer{},
		IndexRepo:      &fakeIndexRepository{savedIndices: map[string]*indexing.InvertedIndex{}},
		ChecksumRepo:   newFakeChecksumRepository(),
		DaemonRepo:     nil,
	})

	err := service.Run()
	if err == nil {
		t.Fatal("expected child directory read error, got nil")
	}
}

func TestInitCommandServiceRunPropagatesFileReadErrorOnIndexBuild(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.readDirMap[rootDir] = []filesystem.DirectoryEntry{{Name: "missing.txt", Path: filepath.Join(rootDir, "missing.txt"), IsDir: false}}

	service := indexing.NewInitCommandService(indexing.InitCommandServiceDeps{
		ProjectTree:    tree,
		MatcherFactory: fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}},
		Output:         &capturingTextOutput{},
		FileReader:     fakeFileReader{files: map[string]string{}},
		Indexer:        &fakeBM25Indexer{},
		IndexRepo:      &fakeIndexRepository{savedIndices: map[string]*indexing.InvertedIndex{}},
		ChecksumRepo:   newFakeChecksumRepository(),
		DaemonRepo:     nil,
	})

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
	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*indexing.InvertedIndex)}
	service := indexing.NewInitCommandService(indexing.InitCommandServiceDeps{
		ProjectTree:    tree,
		MatcherFactory: fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}},
		Output:         output,
		FileReader:     fakeFileReader{files: make(map[string]string)},
		Indexer:        &fakeBM25Indexer{},
		IndexRepo:      indexRepo,
		ChecksumRepo:   newFakeChecksumRepository(),
		DaemonRepo:     nil,
	})

	err := service.Run()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if output.lines[0] != "ℹ️ This project is already indexed. You can run idx search." {
		t.Fatalf("unexpected output message %q", output.lines[0])
	}
}

func TestInitCommandServiceRunCreatesGitIgnoreWithIdxRuleWhenMissing(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true

	service := indexing.NewInitCommandService(indexing.InitCommandServiceDeps{
		ProjectTree:    tree,
		MatcherFactory: fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}},
		Output:         &capturingTextOutput{},
		FileReader:     fakeFileReader{files: map[string]string{}},
		Indexer:        &fakeBM25Indexer{},
		IndexRepo:      &fakeIndexRepository{savedIndices: map[string]*indexing.InvertedIndex{}},
		ChecksumRepo:   newFakeChecksumRepository(),
		DaemonRepo:     nil,
	})

	if err := service.Run(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	gitIgnorePath := filepath.Join(rootDir, ".gitignore")
	if tree.writes[gitIgnorePath] != ".idx/\n" {
		t.Fatalf("expected .gitignore to be created with .idx rule, got %q", tree.writes[gitIgnorePath])
	}
}

func TestInitCommandServiceRunAppendsIdxRuleWhenMissingFromGitIgnore(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true

	gitIgnorePath := filepath.Join(rootDir, ".gitignore")
	service := indexing.NewInitCommandService(indexing.InitCommandServiceDeps{
		ProjectTree:    tree,
		MatcherFactory: fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}},
		Output:         &capturingTextOutput{},
		FileReader:     fakeFileReader{files: map[string]string{gitIgnorePath: "vendor/\n*.log\n"}},
		Indexer:        &fakeBM25Indexer{},
		IndexRepo:      &fakeIndexRepository{savedIndices: map[string]*indexing.InvertedIndex{}},
		ChecksumRepo:   newFakeChecksumRepository(),
		DaemonRepo:     nil,
	})

	if err := service.Run(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := "vendor/\n*.log\n.idx/\n"
	if tree.writes[gitIgnorePath] != expected {
		t.Fatalf("expected .gitignore update %q, got %q", expected, tree.writes[gitIgnorePath])
	}
}

func TestInitCommandServiceRunDoesNotRewriteGitIgnoreWhenRuleAlreadyExists(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true

	gitIgnorePath := filepath.Join(rootDir, ".gitignore")
	service := indexing.NewInitCommandService(indexing.InitCommandServiceDeps{
		ProjectTree:    tree,
		MatcherFactory: fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}},
		Output:         &capturingTextOutput{},
		FileReader:     fakeFileReader{files: map[string]string{gitIgnorePath: "vendor/\n.idx/\n*.log\n"}},
		Indexer:        &fakeBM25Indexer{},
		IndexRepo:      &fakeIndexRepository{savedIndices: map[string]*indexing.InvertedIndex{}},
		ChecksumRepo:   newFakeChecksumRepository(),
		DaemonRepo:     nil,
	})

	if err := service.Run(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if _, wrote := tree.writes[gitIgnorePath]; wrote {
		t.Fatal("expected no .gitignore rewrite when .idx rule already exists")
	}
}
