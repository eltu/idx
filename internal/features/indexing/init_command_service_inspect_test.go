package indexing_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"idx/internal/features/indexing"
	"idx/internal/shared/filesystem"
)

// stubInspectUIRunner captures the index passed to Run for assertion in tests.
type stubInspectUIRunner struct {
	called bool
	index  *indexing.InvertedIndex
	err    error
}

func (s *stubInspectUIRunner) Run(index *indexing.InvertedIndex) error {
	s.called = true
	s.index = index
	return s.err
}

func TestInitCommandService_Inspect_WithoutPath_RunsTUI(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	childDir := filepath.Join(rootDir, "internal")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true
	tree.existing[filepath.Join(childDir, ".idx", "index.idx")] = true
	tree.readDirMap[rootDir] = []filesystem.DirectoryEntry{
		{Name: ".idx", Path: filepath.Join(rootDir, ".idx"), IsDir: true},
		{Name: "internal", Path: childDir, IsDir: true},
	}
	tree.readDirMap[childDir] = []filesystem.DirectoryEntry{}

	matcherFactory := fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}
	output := &capturingTextOutput{}
	fileReader := fakeFileReader{files: map[string]string{}}
	indexer := &fakeBM25Indexer{}
	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*indexing.InvertedIndex)}
	checksumRepo := newFakeChecksumRepository()

	preBuiltIndex := indexing.NewInvertedIndex()
	preBuiltIndex.AddDocument("root.go", filepath.Join(rootDir, "root.go"), 11)
	indexRepo.savedIndices[rootDir] = preBuiltIndex

	childIndex := indexing.NewInvertedIndex()
	childIndex.AddDocument("child.go", filepath.Join(childDir, "child.go"), 7)
	indexRepo.savedIndices[childDir] = childIndex

	stub := &stubInspectUIRunner{}
	service := indexing.NewInitCommandServiceWithInspectUI(indexing.InitCommandServiceDeps{
		ProjectTree:    tree,
		MatcherFactory: matcherFactory,
		Output:         output,
		FileReader:     fileReader,
		Indexer:        indexer,
		IndexRepo:      indexRepo,
		ChecksumRepo:   checksumRepo,
		DaemonRepo:     nil,
	}, stub)

	// Act
	err := service.Inspect("")

	// Assert
	require.NoError(t, err)
	assert.True(t, stub.called, "expected inspect without path to run TUI")
	require.NotNil(t, stub.index)
	assert.Equal(t, 2, stub.index.DocumentCount)
	assert.Len(t, stub.index.Documents, 2)
	assert.Empty(t, output.lines, "expected no JSON output lines in TUI mode")
}

func TestInitCommandService_Inspect_WithoutPath_FailsWhenProjectHasNoIndex(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.readDirMap[rootDir] = []filesystem.DirectoryEntry{}

	service := indexing.NewInitCommandService(indexing.InitCommandServiceDeps{
		ProjectTree:    tree,
		MatcherFactory: fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}},
		Output:         &capturingTextOutput{},
		FileReader:     fakeFileReader{files: map[string]string{}},
		Indexer:        &fakeBM25Indexer{},
		IndexRepo:      &fakeIndexRepository{savedIndices: make(map[string]*indexing.InvertedIndex)},
		ChecksumRepo:   newFakeChecksumRepository(),
		DaemonRepo:     nil,
	})

	// Act
	err := service.Inspect("")

	// Assert
	require.Error(t, err)
}

func TestInitCommandService_Inspect_WithPath_WritesIndexPayload(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true
	matcherFactory := fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}
	output := &capturingTextOutput{}
	fileReader := fakeFileReader{files: map[string]string{
		filepath.Join(rootDir, "allowed.txt"): "hello world test",
	}}
	indexer := &fakeBM25Indexer{}
	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*indexing.InvertedIndex)}
	checksumRepo := newFakeChecksumRepository()

	tree.readDirMap[rootDir] = []filesystem.DirectoryEntry{
		{Name: "allowed.txt", Path: filepath.Join(rootDir, "allowed.txt"), IsDir: false},
	}

	preBuiltIndex := indexing.NewInvertedIndex()
	preBuiltIndex.DocumentCount = 1
	indexRepo.savedIndices[rootDir] = preBuiltIndex

	service := indexing.NewInitCommandService(indexing.InitCommandServiceDeps{
		ProjectTree:    tree,
		MatcherFactory: matcherFactory,
		Output:         output,
		FileReader:     fileReader,
		Indexer:        indexer,
		IndexRepo:      indexRepo,
		ChecksumRepo:   checksumRepo,
		DaemonRepo:     nil,
	})

	// Act
	err := service.Inspect(".")

	// Assert
	require.NoError(t, err)
	require.Len(t, output.lines, 1)
	var payload indexing.InvertedIndex
	require.NoError(t, json.Unmarshal([]byte(output.lines[0]), &payload))
	assert.Equal(t, 1, payload.DocumentCount)
}

func TestInitCommandService_Inspect_FailsWhenNoIndexExists(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := newFakeProjectTree(rootDir, rootDir)
	matcherFactory := fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}
	output := &capturingTextOutput{}
	fileReader := fakeFileReader{files: make(map[string]string)}
	indexer := &fakeBM25Indexer{}
	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*indexing.InvertedIndex)}
	checksumRepo := newFakeChecksumRepository()
	service := indexing.NewInitCommandService(indexing.InitCommandServiceDeps{
		ProjectTree:    tree,
		MatcherFactory: matcherFactory,
		Output:         output,
		FileReader:     fileReader,
		Indexer:        indexer,
		IndexRepo:      indexRepo,
		ChecksumRepo:   checksumRepo,
		DaemonRepo:     nil,
	})

	// Act
	err := service.Inspect("internal")

	// Assert
	require.Error(t, err)
	assert.Empty(t, indexRepo.savedIndices, "expected no index saves (inspect is read-only)")
	assert.Empty(t, output.lines, "expected no output lines on failure")
}

func TestInitCommandService_Inspect_LoadsNestedIndexFromRelativePath(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	nestedDir := filepath.Join(rootDir, "internal")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(nestedDir, ".idx", "index.idx")] = true
	matcherFactory := fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}}
	output := &capturingTextOutput{}
	fileReader := fakeFileReader{files: make(map[string]string)}
	indexer := &fakeBM25Indexer{}
	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*indexing.InvertedIndex)}
	checksumRepo := newFakeChecksumRepository()

	preBuiltIndex := indexing.NewInvertedIndex()
	preBuiltIndex.DocumentCount = 2
	indexRepo.savedIndices[nestedDir] = preBuiltIndex

	service := indexing.NewInitCommandService(indexing.InitCommandServiceDeps{
		ProjectTree:    tree,
		MatcherFactory: matcherFactory,
		Output:         output,
		FileReader:     fileReader,
		Indexer:        indexer,
		IndexRepo:      indexRepo,
		ChecksumRepo:   checksumRepo,
		DaemonRepo:     nil,
	})

	// Act
	err := service.Inspect("internal/")

	// Assert
	require.NoError(t, err)
	require.Len(t, output.lines, 1)
	var payload indexing.InvertedIndex
	require.NoError(t, json.Unmarshal([]byte(output.lines[0]), &payload))
	assert.Equal(t, 2, payload.DocumentCount)
}
