package indexing

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"idx/internal/shared/filesystem"
)

func TestTrackEventDirectories_AddsPathForTrackedEvent(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	subdir := filepath.Join(root, "pkg")
	require.NoError(t, os.Mkdir(subdir, 0755))
	service := newWatchService(root)
	pending := make(map[string]struct{})

	// Act
	service.trackEventDirectories(fsnotify.Event{Op: fsnotify.Write, Name: filepath.Join(subdir, "file.go")}, root, neverMatcher{}, pending)

	// Assert
	_, ok := pending[subdir]
	assert.True(t, ok, "expected subdir to be added to pending")
}

func TestTrackEventDirectories_SkipsNonTrackedOp(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	service := newWatchService(root)
	pending := make(map[string]struct{})

	// Act
	service.trackEventDirectories(fsnotify.Event{Op: fsnotify.Chmod, Name: filepath.Join(root, "file.go")}, root, neverMatcher{}, pending)

	// Assert
	assert.Empty(t, pending)
}

func TestTrackEventFiles_AddsRelativePathForExistingFile(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	file := filepath.Join(root, "main.go")
	require.NoError(t, os.WriteFile(file, []byte("package main"), 0644))
	service := newWatchService(root)
	pending := make(map[string]struct{})

	// Act
	service.trackEventFiles(fsnotify.Event{Op: fsnotify.Write, Name: file}, root, neverMatcher{}, pending)

	// Assert
	_, ok := pending["main.go"]
	assert.True(t, ok, "expected main.go in pending files")
}

func TestTrackEventFiles_SkipsNonTrackedOp(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	service := newWatchService(root)
	pending := make(map[string]struct{})

	// Act
	service.trackEventFiles(fsnotify.Event{Op: fsnotify.Chmod, Name: filepath.Join(root, "file.go")}, root, neverMatcher{}, pending)

	// Assert
	assert.Empty(t, pending)
}

func TestWriteUpdatedFiles_WithFiles(t *testing.T) {
	t.Parallel()

	// Arrange
	out := &internalWatchOutput{}
	service := newWatchService(t.TempDir())
	service.output = out

	// Act
	require.NoError(t, service.writeUpdatedFiles(map[string]struct{}{"internal/service.go": {}, "cmd/main.go": {}}))

	// Assert
	assert.NotEmpty(t, out.lines)
}

func TestWriteUpdatedFiles_WithEmpty(t *testing.T) {
	t.Parallel()

	// Arrange
	out := &internalWatchOutput{}
	service := newWatchService(t.TempDir())
	service.output = out

	// Act
	require.NoError(t, service.writeUpdatedFiles(map[string]struct{}{}))

	// Assert
	require.Len(t, out.lines, 1)
	assert.Equal(t, "   files: none", out.lines[0])
}

func TestFlushWatchedBatch_EmptyDirectories_IsNoop(t *testing.T) {
	t.Parallel()

	// Arrange
	out := &internalWatchOutput{}
	service := newWatchService(t.TempDir())
	service.output = out

	// Act
	err := service.flushWatchedBatch(map[string]struct{}{}, map[string]struct{}{}, t.TempDir(), neverMatcher{}, false)

	// Assert
	require.NoError(t, err)
	assert.Empty(t, out.lines)
}

func TestEnsureRootIndex_CreatesIndexWhenAbsent(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	out := &internalWatchOutput{}
	service := newWatchService(root)
	service.output = out

	// Act
	require.NoError(t, service.ensureRootIndex(root, neverMatcher{}))

	// Assert
	assert.NotEmpty(t, out.lines)
}

func TestEnsureRootIndex_SkipsWhenIndexExists(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	out := &internalWatchOutput{}
	tree := &internalWatchExistsProjectTree{root: root}
	service := newWatchService(root)
	service.projectTree = tree
	service.output = out

	// Act
	require.NoError(t, service.ensureRootIndex(root, neverMatcher{}))

	// Assert
	assert.Empty(t, out.lines)
}

type internalWatchExistsProjectTree struct{ root string }

func (t *internalWatchExistsProjectTree) CurrentDir() (string, error)          { return t.root, nil }
func (t *internalWatchExistsProjectTree) FindGitRoot(_ string) (string, error) { return t.root, nil }
func (t *internalWatchExistsProjectTree) ReadDir(_ string) ([]filesystem.DirectoryEntry, error) {
	return nil, nil
}
func (t *internalWatchExistsProjectTree) Exists(_ string) (bool, error)      { return true, nil }
func (t *internalWatchExistsProjectTree) WriteFile(_ string, _ []byte) error { return nil }
func (t *internalWatchExistsProjectTree) RemoveAll(_ string) error           { return nil }

func TestSyncAllDirectoriesBeforeWatch_SyncsEligibleAndRemovesStale(t *testing.T) {
	t.Parallel()

	// Arrange
	root := filepath.Join(string(filepath.Separator), "repo")
	sourceDir := filepath.Join(root, "src")
	ignoredDir := filepath.Join(root, "vendor")
	rootFile := filepath.Join(root, "main.go")
	sourceFile := filepath.Join(sourceDir, "service.go")

	tree := newInternalProjectTree(root)
	tree.existsMap[indexFilePath(root)] = true
	tree.existsMap[indexFilePath(sourceDir)] = false
	tree.existsMap[indexFilePath(ignoredDir)] = true
	tree.readDirMap[root] = []filesystem.DirectoryEntry{
		{Name: "src", Path: sourceDir, IsDir: true},
		{Name: "vendor", Path: ignoredDir, IsDir: true},
		{Name: "main.go", Path: rootFile, IsDir: false, Size: int64(len("package main")), ModTimeUnixNano: 1},
	}
	tree.readDirMap[sourceDir] = []filesystem.DirectoryEntry{
		{Name: "service.go", Path: sourceFile, IsDir: false, Size: int64(len("package service")), ModTimeUnixNano: 2},
	}
	tree.readDirMap[ignoredDir] = []filesystem.DirectoryEntry{
		{Name: "legacy.go", Path: filepath.Join(ignoredDir, "legacy.go"), IsDir: false},
	}

	matcher := watchStartupMatcher{ignoredPrefixes: []string{"vendor"}}
	indexRepo := &watchStartupIndexRepo{}
	checksumRepo := &watchStartupChecksumRepo{}

	service := InitCommandService{
		projectTree:    tree,
		matcherFactory: internalMatcherFactory{matcher: matcher},
		output:         internalOutput{},
		fileReader:     internalFileReader{files: map[string]string{rootFile: "package main", sourceFile: "package service"}},
		indexer:        internalIndexer{},
		indexRepo:      indexRepo,
		checksumRepo:   checksumRepo,
		inspectUI:      internalInspectUIRunner{},
		initProgress:   disabledInitProgress{},
	}

	// Act
	require.NoError(t, service.syncAllDirectoriesBeforeWatch(root, matcher))

	// Assert
	assert.True(t, containsPath(tree.removed, filepath.Join(ignoredDir, ".idx")), "expected stale ignored directory index to be removed")
	assert.True(t, containsPath(indexRepo.savedDirectories, root), "expected root directory to be synchronized")
	assert.True(t, containsPath(indexRepo.savedDirectories, sourceDir), "expected source directory to be synchronized")
	assert.False(t, containsPath(indexRepo.savedDirectories, ignoredDir), "expected ignored directory to be skipped")
}

type watchStartupMatcher struct{ ignoredPrefixes []string }

func (matcher watchStartupMatcher) Matches(path string) (bool, error) {
	for _, prefix := range matcher.ignoredPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true, nil
		}
	}
	return false, nil
}

type watchStartupIndexRepo struct {
	mu               sync.Mutex
	savedDirectories []string
}

func (repo *watchStartupIndexRepo) SaveIndex(directoryPath string, _ *InvertedIndex) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	repo.savedDirectories = append(repo.savedDirectories, directoryPath)
	return nil
}

func (repo *watchStartupIndexRepo) LoadIndex(_ string) (*InvertedIndex, error) {
	return NewInvertedIndex(), nil
}

type watchStartupChecksumRepo struct {
	mu       sync.RWMutex
	loadData map[string]map[string]string
}

func (repo *watchStartupChecksumRepo) Load(directoryPath string) (map[string]string, bool, error) {
	repo.mu.RLock()
	defer repo.mu.RUnlock()
	if repo.loadData == nil {
		return map[string]string{}, false, nil
	}
	checksums, exists := repo.loadData[directoryPath]
	if !exists {
		return map[string]string{}, false, nil
	}
	return checksums, true, nil
}

func (repo *watchStartupChecksumRepo) Save(directoryPath string, checksums map[string]string) error {
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if repo.loadData == nil {
		repo.loadData = map[string]map[string]string{}
	}
	repo.loadData[directoryPath] = checksums
	return nil
}

func containsPath(paths []string, target string) bool {
	for _, current := range paths {
		if current == target {
			return true
		}
	}
	return false
}
