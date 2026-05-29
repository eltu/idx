package indexing_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"idx/internal/features/indexing"
	"idx/internal/shared/filesystem"
)

func TestInitCommandService_Run_WritesIndexFilesForAllowedEntries(t *testing.T) {
	t.Parallel()

	// Arrange
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

	// Act
	err := service.Run()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 2, indexRepo.savedIndices[rootDir].DocumentCount)
	assert.Equal(t, 1, indexRepo.savedIndices[childDir].DocumentCount)
	assert.NotContains(t, indexRepo.savedIndices, emptyDir)
	assert.NotContains(t, indexRepo.savedIndices, vendorDir)
}

func TestInitCommandService_Run_RejectsDirectoryOutsideGitProject(t *testing.T) {
	t.Parallel()

	// Arrange
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

	// Act
	err := service.Run()

	// Assert
	require.Error(t, err)
}

func TestInitCommandService_Run_ReturnsCurrentDirError(t *testing.T) {
	t.Parallel()

	// Arrange
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

	// Act
	err := service.Run()

	// Assert
	require.Error(t, err)
}

func TestInitCommandService_Inspect_ReturnsCurrentDirError(t *testing.T) {
	t.Parallel()

	// Arrange
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

	// Act
	err := service.Inspect(".")

	// Assert
	require.Error(t, err)
}

func TestInitCommandService_Run_ReturnsErrorWhenDependenciesAreNil(t *testing.T) {
	t.Parallel()

	// Arrange
	service := indexing.NewInitCommandService(indexing.InitCommandServiceDeps{})

	// Act
	err := service.Run()

	// Assert
	require.Error(t, err)
}

func TestInitCommandService_Inspect_ReturnsErrorWhenDependenciesAreNil(t *testing.T) {
	t.Parallel()

	// Arrange
	service := indexing.NewInitCommandService(indexing.InitCommandServiceDeps{})

	// Act
	err := service.Inspect(".")

	// Assert
	require.Error(t, err)
}

func TestInitCommandService_Run_ReturnsMatcherFactoryError(t *testing.T) {
	t.Parallel()

	// Arrange
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

	// Act
	err := service.Run()

	// Assert
	require.Error(t, err)
}

func TestInitCommandService_Run_PropagatesChildDirectoryReadError(t *testing.T) {
	t.Parallel()

	// Arrange
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

	// Act
	err := service.Run()

	// Assert
	require.Error(t, err)
}

func TestInitCommandService_Run_PropagatesFileReadErrorOnIndexBuild(t *testing.T) {
	t.Parallel()

	// Arrange
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

	// Act
	err := service.Run()

	// Assert
	require.Error(t, err)
}

func TestInitCommandService_Run_SkipsWhenIndexAlreadyExists(t *testing.T) {
	t.Parallel()

	// Arrange
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

	// Act
	err := service.Run()

	// Assert
	require.NoError(t, err)
	require.Equal(t, "ℹ️ This project is already indexed. You can run idx search.", output.lines[0])
}

func TestInitCommandService_Run_CreatesGitIgnoreWithIdxRuleWhenMissing(t *testing.T) {
	t.Parallel()

	// Arrange
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

	// Act
	err := service.Run()

	// Assert
	require.NoError(t, err)
	gitIgnorePath := filepath.Join(rootDir, ".gitignore")
	assert.Equal(t, ".idx/\n", tree.writes[gitIgnorePath])
}

func TestInitCommandService_Run_AppendsIdxRuleWhenMissingFromGitIgnore(t *testing.T) {
	t.Parallel()

	// Arrange
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

	// Act
	err := service.Run()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "vendor/\n*.log\n.idx/\n", tree.writes[gitIgnorePath])
}

func TestInitCommandService_Run_DoesNotRewriteGitIgnoreWhenRuleAlreadyExists(t *testing.T) {
	t.Parallel()

	// Arrange
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

	// Act
	err := service.Run()

	// Assert
	require.NoError(t, err)
	assert.NotContains(t, tree.writes, gitIgnorePath)
}
