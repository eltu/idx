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

func TestInitCommandService_Sync_ReturnsCurrentDirError(t *testing.T) {
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

	// Act & Assert
	require.Error(t, service.Sync())
}

func TestInitCommandService_Sync_ReturnsErrorWhenDependenciesAreNil(t *testing.T) {
	t.Parallel()

	// Arrange
	service := indexing.NewInitCommandService(indexing.InitCommandServiceDeps{})

	// Act & Assert
	require.Error(t, service.Sync())
}

func TestInitCommandService_Sync_ReturnsMatcherFactoryError(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true
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

	// Act & Assert
	require.Error(t, service.Sync())
}

func TestInitCommandService_Sync_RebuildsAllIndexedDirectories(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	childDir := filepath.Join(rootDir, "child")
	otherDir := filepath.Join(rootDir, "other")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true
	tree.existing[filepath.Join(childDir, ".idx", "index.idx")] = true
	tree.readDirMap[rootDir] = []filesystem.DirectoryEntry{
		{Name: ".idx", Path: filepath.Join(rootDir, ".idx"), IsDir: true},
		{Name: "root.txt", Path: filepath.Join(rootDir, "root.txt"), IsDir: false},
		{Name: "child", Path: childDir, IsDir: true},
		{Name: "other", Path: otherDir, IsDir: true},
	}
	tree.readDirMap[childDir] = []filesystem.DirectoryEntry{{Name: "nested.txt", Path: filepath.Join(childDir, "nested.txt"), IsDir: false}}
	tree.readDirMap[otherDir] = []filesystem.DirectoryEntry{{Name: "other.txt", Path: filepath.Join(otherDir, "other.txt"), IsDir: false}}
	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*indexing.InvertedIndex)}
	service := indexing.NewInitCommandService(indexing.InitCommandServiceDeps{
		ProjectTree:    tree,
		MatcherFactory: fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}},
		Output:         &capturingTextOutput{},
		FileReader: fakeFileReader{files: map[string]string{
			filepath.Join(rootDir, "root.txt"):    "root content",
			filepath.Join(childDir, "nested.txt"): "nested content",
			filepath.Join(otherDir, "other.txt"):  "other content",
		}},
		Indexer:      &fakeBM25Indexer{},
		IndexRepo:    indexRepo,
		ChecksumRepo: newFakeChecksumRepository(),
		DaemonRepo:   nil,
	})

	// Act
	require.NoError(t, service.Sync())

	// Assert
	assert.Equal(t, 1, indexRepo.savedIndices[rootDir].DocumentCount)
	assert.Equal(t, 1, indexRepo.savedIndices[childDir].DocumentCount)
	assert.Equal(t, 1, indexRepo.savedIndices[otherDir].DocumentCount)
}

func TestInitCommandService_Sync_SkipsReindexWhenChecksumsUnchanged(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	filePath := filepath.Join(rootDir, "root.txt")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true
	tree.readDirMap[rootDir] = []filesystem.DirectoryEntry{
		{Name: ".idx", Path: filepath.Join(rootDir, ".idx"), IsDir: true},
		{Name: "root.txt", Path: filePath, IsDir: false},
	}
	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*indexing.InvertedIndex)}
	checksumRepo := newFakeChecksumRepository()
	checksumRepo.exists[rootDir] = true
	checksumRepo.checksums[rootDir] = map[string]string{"root.txt": checksumFromContent("root content")}
	service := indexing.NewInitCommandService(indexing.InitCommandServiceDeps{
		ProjectTree:    tree,
		MatcherFactory: fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}},
		Output:         &capturingTextOutput{},
		FileReader:     fakeFileReader{files: map[string]string{filePath: "root content"}},
		Indexer:        &fakeBM25Indexer{},
		IndexRepo:      indexRepo,
		ChecksumRepo:   checksumRepo,
		DaemonRepo:     nil,
	})

	// Act
	require.NoError(t, service.Sync())

	// Assert
	assert.Empty(t, indexRepo.savedIndices, "expected no reindex saves for unchanged checksum")
}

func TestInitCommandService_Sync_ReindexesWhenChecksumsChanged(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	filePath := filepath.Join(rootDir, "root.txt")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true
	tree.readDirMap[rootDir] = []filesystem.DirectoryEntry{
		{Name: ".idx", Path: filepath.Join(rootDir, ".idx"), IsDir: true},
		{Name: "root.txt", Path: filePath, IsDir: false},
	}
	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*indexing.InvertedIndex)}
	checksumRepo := newFakeChecksumRepository()
	checksumRepo.exists[rootDir] = true
	checksumRepo.checksums[rootDir] = map[string]string{"root.txt": checksumFromContent("old root content")}
	service := indexing.NewInitCommandService(indexing.InitCommandServiceDeps{
		ProjectTree:    tree,
		MatcherFactory: fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}},
		Output:         &capturingTextOutput{},
		FileReader:     fakeFileReader{files: map[string]string{filePath: "new root content"}},
		Indexer:        &fakeBM25Indexer{},
		IndexRepo:      indexRepo,
		ChecksumRepo:   checksumRepo,
		DaemonRepo:     nil,
	})

	// Act
	require.NoError(t, service.Sync())

	// Assert
	assert.Equal(t, 1, checksumRepo.saveCount[rootDir])
}

func TestInitCommandService_Sync_RemovesIndexWhenDirectoryHasNoIndexableFiles(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true
	tree.readDirMap[rootDir] = []filesystem.DirectoryEntry{{Name: ".idx", Path: filepath.Join(rootDir, ".idx"), IsDir: true}}
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
	require.NoError(t, service.Sync())

	// Assert
	require.Len(t, tree.removed, 1)
	assert.Equal(t, filepath.Join(rootDir, ".idx"), tree.removed[0])
}

func TestInitCommandService_Sync_RemovesIndexForNowIgnoredDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	ignoredDir := filepath.Join(rootDir, "vendor")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true
	tree.existing[filepath.Join(ignoredDir, ".idx", "index.idx")] = true
	tree.readDirMap[rootDir] = []filesystem.DirectoryEntry{
		{Name: ".idx", Path: filepath.Join(rootDir, ".idx"), IsDir: true},
		{Name: "root.txt", Path: filepath.Join(rootDir, "root.txt"), IsDir: false},
		{Name: "vendor", Path: ignoredDir, IsDir: true},
	}
	tree.readDirMap[ignoredDir] = []filesystem.DirectoryEntry{{Name: "dep.txt", Path: filepath.Join(ignoredDir, "dep.txt"), IsDir: false}}
	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*indexing.InvertedIndex)}
	service := indexing.NewInitCommandService(indexing.InitCommandServiceDeps{
		ProjectTree:    tree,
		MatcherFactory: fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{"vendor/": true}},
		Output:         &capturingTextOutput{},
		FileReader:     fakeFileReader{files: map[string]string{filepath.Join(rootDir, "root.txt"): "root content"}},
		Indexer:        &fakeBM25Indexer{},
		IndexRepo:      indexRepo,
		ChecksumRepo:   newFakeChecksumRepository(),
		DaemonRepo:     nil,
	})

	// Act
	require.NoError(t, service.Sync())

	// Assert
	require.Len(t, tree.removed, 1)
	assert.Equal(t, filepath.Join(ignoredDir, ".idx"), tree.removed[0])
	_, reindexed := indexRepo.savedIndices[ignoredDir]
	assert.False(t, reindexed, "expected ignored directory not to be reindexed")
}

func TestInitCommandService_Sync_CreatesIndexWhenDirectoryLeavesGitignore(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	newlyAllowedDir := filepath.Join(rootDir, "src")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true
	tree.readDirMap[rootDir] = []filesystem.DirectoryEntry{
		{Name: ".idx", Path: filepath.Join(rootDir, ".idx"), IsDir: true},
		{Name: "root.txt", Path: filepath.Join(rootDir, "root.txt"), IsDir: false},
		{Name: "src", Path: newlyAllowedDir, IsDir: true},
	}
	tree.readDirMap[newlyAllowedDir] = []filesystem.DirectoryEntry{{Name: "app.txt", Path: filepath.Join(newlyAllowedDir, "app.txt"), IsDir: false}}
	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*indexing.InvertedIndex)}
	service := indexing.NewInitCommandService(indexing.InitCommandServiceDeps{
		ProjectTree:    tree,
		MatcherFactory: fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}},
		Output:         &capturingTextOutput{},
		FileReader: fakeFileReader{files: map[string]string{
			filepath.Join(rootDir, "root.txt"):        "root content",
			filepath.Join(newlyAllowedDir, "app.txt"): "source content",
		}},
		Indexer:      &fakeBM25Indexer{},
		IndexRepo:    indexRepo,
		ChecksumRepo: newFakeChecksumRepository(),
		DaemonRepo:   nil,
	})

	// Act
	require.NoError(t, service.Sync())

	// Assert
	assert.Equal(t, 1, indexRepo.savedIndices[newlyAllowedDir].DocumentCount)
}

func TestInitCommandService_Sync_ReindexesDirectoryWhenFileLeavesGitignore(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true
	tree.readDirMap[rootDir] = []filesystem.DirectoryEntry{
		{Name: ".idx", Path: filepath.Join(rootDir, ".idx"), IsDir: true},
		{Name: "allowed.txt", Path: filepath.Join(rootDir, "allowed.txt"), IsDir: false},
		{Name: "debug.log", Path: filepath.Join(rootDir, "debug.log"), IsDir: false},
	}
	indexRepo := &fakeIndexRepository{savedIndices: make(map[string]*indexing.InvertedIndex)}
	service := indexing.NewInitCommandService(indexing.InitCommandServiceDeps{
		ProjectTree:    tree,
		MatcherFactory: fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}},
		Output:         &capturingTextOutput{},
		FileReader: fakeFileReader{files: map[string]string{
			filepath.Join(rootDir, "allowed.txt"): "allowed content",
			filepath.Join(rootDir, "debug.log"):   "log content",
		}},
		Indexer:      &fakeBM25Indexer{},
		IndexRepo:    indexRepo,
		ChecksumRepo: newFakeChecksumRepository(),
		DaemonRepo:   nil,
	})

	// Act
	require.NoError(t, service.Sync())

	// Assert
	assert.Equal(t, 2, indexRepo.savedIndices[rootDir].DocumentCount)
}

func TestInitCommandService_Sync_RequiresProjectRootIndex(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	tree := newFakeProjectTree(rootDir, rootDir)
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

	// Act & Assert
	require.Error(t, service.Sync())
}

func TestInitCommandService_Sync_MustRunFromProjectRoot(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	childDir := filepath.Join(rootDir, "child")
	tree := newFakeProjectTree(childDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true
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

	// Act & Assert
	require.Error(t, service.Sync())
}

func TestInitCommandService_Sync_SkipsRehashWhenMetadataUnchanged(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	filePath := filepath.Join(rootDir, "root.txt")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true
	tree.readDirMap[rootDir] = []filesystem.DirectoryEntry{
		{Name: ".idx", Path: filepath.Join(rootDir, ".idx"), IsDir: true},
		{Name: "root.txt", Path: filePath, IsDir: false, Size: int64(len("root content")), ModTimeUnixNano: 42},
	}
	reader := &countingFileReader{files: map[string]string{filePath: "root content"}}
	checksumRepo := newFakeChecksumRepository()
	checksumRepo.exists[rootDir] = true
	checksumRepo.snapshots[rootDir] = indexing.DirectoryChecksumSnapshot{Files: map[string]indexing.FileChecksumState{
		"root.txt": {Checksum: checksumFromContent("root content"), Size: int64(len("root content")), ModTimeUnixNano: 42},
	}}
	service := indexing.NewInitCommandService(indexing.InitCommandServiceDeps{
		ProjectTree:    tree,
		MatcherFactory: fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}},
		Output:         &capturingTextOutput{},
		FileReader:     reader,
		Indexer:        &fakeBM25Indexer{},
		IndexRepo:      &fakeIndexRepository{savedIndices: make(map[string]*indexing.InvertedIndex)},
		ChecksumRepo:   checksumRepo,
		DaemonRepo:     nil,
	})

	// Act
	require.NoError(t, service.Sync())

	// Assert
	assert.Equal(t, 0, reader.readCount, "expected 0 file reads when metadata is unchanged")
}

func TestInitCommandService_Sync_RehashesWhenMetadataChanges(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	filePath := filepath.Join(rootDir, "root.txt")
	tree := newFakeProjectTree(rootDir, rootDir)
	tree.existing[filepath.Join(rootDir, ".idx", "index.idx")] = true
	tree.readDirMap[rootDir] = []filesystem.DirectoryEntry{
		{Name: ".idx", Path: filepath.Join(rootDir, ".idx"), IsDir: true},
		{Name: "root.txt", Path: filePath, IsDir: false, Size: int64(len("new content")), ModTimeUnixNano: 200},
	}
	reader := &countingFileReader{files: map[string]string{filePath: "new content"}}
	checksumRepo := newFakeChecksumRepository()
	checksumRepo.exists[rootDir] = true
	checksumRepo.snapshots[rootDir] = indexing.DirectoryChecksumSnapshot{Files: map[string]indexing.FileChecksumState{
		"root.txt": {Checksum: checksumFromContent("old content"), Size: int64(len("old content")), ModTimeUnixNano: 100},
	}}
	service := indexing.NewInitCommandService(indexing.InitCommandServiceDeps{
		ProjectTree:    tree,
		MatcherFactory: fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}},
		Output:         &capturingTextOutput{},
		FileReader:     reader,
		Indexer:        &fakeBM25Indexer{},
		IndexRepo:      &fakeIndexRepository{savedIndices: make(map[string]*indexing.InvertedIndex)},
		ChecksumRepo:   checksumRepo,
		DaemonRepo:     nil,
	})

	// Act
	require.NoError(t, service.Sync())

	// Assert
	assert.Positive(t, reader.readCount, "expected at least one file read when metadata changes")
}
