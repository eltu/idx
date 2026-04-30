package indexing_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"idx/internal/core/domain"
	"idx/internal/core/services/indexing"
)

func TestInitCommandServiceInspectWithoutPathRunsTUI(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	childDir := filepath.Join(rootDir, "internal")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true
	tree.existing[filepath.Join(childDir, ".idx", "index.idx")] = true
	tree.readDirMap[rootDir] = []domain.DirectoryEntry{
		{Name: ".idx", Path: filepath.Join(rootDir, ".idx"), IsDir: true},
		{Name: "internal", Path: childDir, IsDir: true},
	}
	tree.readDirMap[childDir] = []domain.DirectoryEntry{}

	matcherFactory := fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}
	output := &capturingTextOutput{}
	fileReader := fakeFileReader{files: map[string]string{}}
	indexer := &fakeBM25Indexer{}
	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*domain.InvertedIndex)}
	checksumRepo := newFakeChecksumRepository()

	preBuiltIndex := domain.NewInvertedIndex()
	preBuiltIndex.AddDocument("root.go", filepath.Join(rootDir, "root.go"), 11)
	indexRepo.savedIndices[rootDir] = preBuiltIndex

	childIndex := domain.NewInvertedIndex()
	childIndex.AddDocument("child.go", filepath.Join(childDir, "child.go"), 7)
	indexRepo.savedIndices[childDir] = childIndex

	tuiCalled := false
	originalRunInspectTUI := indexing.RunInspectTUITestHook()
	indexing.SetRunInspectTUITestHook(func(index *domain.InvertedIndex) error {
		tuiCalled = true
		if index == nil || index.DocumentCount != 2 {
			t.Fatalf("expected inspect TUI to receive merged project indices, got %+v", index)
		}

		if len(index.Documents) != 2 {
			t.Fatalf("expected merged inspect index with 2 documents, got %d", len(index.Documents))
		}
		return nil
	})
	t.Cleanup(func() {
		indexing.SetRunInspectTUITestHook(originalRunInspectTUI)
	})

	service := indexing.NewInitCommandService(tree, matcherFactory, output, fileReader, indexer, indexRepo, checksumRepo, nil)

	err := service.Inspect("")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !tuiCalled {
		t.Fatal("expected inspect without path to run TUI")
	}

	if len(output.lines) != 0 {
		t.Fatalf("expected no JSON output lines in TUI mode, got %d", len(output.lines))
	}
}

func TestInitCommandServiceInspectWithoutPathFailsWhenProjectHasNoIndex(t *testing.T) {
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.readDirMap[rootDir] = []domain.DirectoryEntry{}

	service := indexing.NewInitCommandService(
		tree,
		fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}},
		&capturingTextOutput{},
		fakeFileReader{files: map[string]string{}},
		&fakeBM25Indexer{},
		&fakeIndexRepository{savedIndices: make(map[string]*domain.InvertedIndex)},
		newFakeChecksumRepository(),
		nil,
	)

	err := service.Inspect("")
	if err == nil {
		t.Fatal("expected an error when project has no indexed directories")
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

	service := indexing.NewInitCommandService(tree, matcherFactory, output, fileReader, indexer, indexRepo, checksumRepo, nil)

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
	service := indexing.NewInitCommandService(tree, matcherFactory, output, fileReader, indexer, indexRepo, checksumRepo, nil)

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

	service := indexing.NewInitCommandService(tree, matcherFactory, output, fileReader, indexer, indexRepo, checksumRepo, nil)

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
