package indexing

import (
	"errors"
	"math"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"idx/internal/shared/filesystem"
)

type internalProjectTree struct {
	currentDir string
	currentErr error
	gitRoot    string
	gitRootErr error
	readDirMap map[string][]filesystem.DirectoryEntry
	readDirErr map[string]error
	existsMap  map[string]bool
	existsErr  map[string]error
	removed    []string
	removeErr  error
}

func newInternalProjectTree(root string) *internalProjectTree {
	return &internalProjectTree{
		currentDir: root,
		gitRoot:    root,
		readDirMap: map[string][]filesystem.DirectoryEntry{},
		readDirErr: map[string]error{},
		existsMap:  map[string]bool{},
		existsErr:  map[string]error{},
		removed:    []string{},
	}
}

func (tree *internalProjectTree) CurrentDir() (string, error) {
	if tree.currentErr != nil {
		return "", tree.currentErr
	}

	return tree.currentDir, nil
}

func (tree *internalProjectTree) FindGitRoot(_ string) (string, error) {
	if tree.gitRootErr != nil {
		return "", tree.gitRootErr
	}

	return tree.gitRoot, nil
}

func (tree *internalProjectTree) ReadDir(path string) ([]filesystem.DirectoryEntry, error) {
	if err, ok := tree.readDirErr[path]; ok {
		return nil, err
	}

	entries, ok := tree.readDirMap[path]
	if !ok {
		return []filesystem.DirectoryEntry{}, nil
	}

	return entries, nil
}

func (tree *internalProjectTree) Exists(path string) (bool, error) {
	if err, ok := tree.existsErr[path]; ok {
		return false, err
	}

	return tree.existsMap[path], nil
}

func (tree *internalProjectTree) RemoveAll(path string) error {
	tree.removed = append(tree.removed, path)
	if tree.removeErr != nil {
		return tree.removeErr
	}

	return nil
}

func (*internalProjectTree) WriteFile(string, []byte) error { return nil }

type internalMatcherFactory struct {
	matcher filesystem.IgnoreMatcher
	err     error
}

func (factory internalMatcherFactory) New(_ string) (filesystem.IgnoreMatcher, error) {
	if factory.err != nil {
		return nil, factory.err
	}

	return factory.matcher, nil
}

type internalMatcher struct {
	ignored bool
	err     error
}

func (matcher internalMatcher) Matches(_ string) (bool, error) {
	if matcher.err != nil {
		return false, matcher.err
	}

	return matcher.ignored, nil
}

type countingMatcher struct {
	calls int
}

func (matcher *countingMatcher) Matches(_ string) (bool, error) {
	matcher.calls++
	return false, nil
}

type internalOutput struct{}

func (internalOutput) WriteLine(string) error { return nil }

type internalInspectUIRunner struct{}

func (internalInspectUIRunner) Run(_ *InvertedIndex) error { return nil }

type internalFileReader struct {
	files map[string]string
	err   error
}

func (reader internalFileReader) ReadFile(path string) (string, error) {
	if reader.err != nil {
		return "", reader.err
	}

	content, ok := reader.files[path]
	if !ok {
		return "", errors.New("file not found")
	}

	return content, nil
}

type internalIndexer struct {
	err error
}

func (indexer internalIndexer) BuildIndex(_ []IndexDocument) (*InvertedIndex, error) {
	if indexer.err != nil {
		return nil, indexer.err
	}

	return NewInvertedIndex(), nil
}

type internalIndexRepo struct {
	loadErr error
	saveErr error
	index   *InvertedIndex
}

func (repo internalIndexRepo) LoadIndex(_ string) (*InvertedIndex, error) {
	if repo.loadErr != nil {
		return nil, repo.loadErr
	}

	if repo.index != nil {
		return repo.index, nil
	}

	return NewInvertedIndex(), nil
}

func (repo internalIndexRepo) SaveIndex(_ string, _ *InvertedIndex) error {
	if repo.saveErr != nil {
		return repo.saveErr
	}

	return nil
}

type internalChecksumRepo struct {
	loadData map[string]string
	exists   bool
	loadErr  error
	saveErr  error
}

func (repo internalChecksumRepo) Load(_ string) (map[string]string, bool, error) {
	if repo.loadErr != nil {
		return nil, false, repo.loadErr
	}

	if repo.loadData == nil {
		return map[string]string{}, repo.exists, nil
	}

	return repo.loadData, repo.exists, nil
}

func (repo internalChecksumRepo) Save(_ string, _ map[string]string) error {
	if repo.saveErr != nil {
		return repo.saveErr
	}

	return nil
}

type internalSnapshotChecksumRepo struct {
	loadSnapshotResult DirectoryChecksumSnapshot
	loadSnapshotExists bool
	loadSnapshotErr    error
	saveSnapshotErr    error
}

func (repo internalSnapshotChecksumRepo) Load(string) (map[string]string, bool, error) {
	return map[string]string{}, false, nil
}

func (repo internalSnapshotChecksumRepo) Save(string, map[string]string) error {
	return nil
}

func (repo internalSnapshotChecksumRepo) LoadSnapshot(string) (DirectoryChecksumSnapshot, bool, error) {
	return repo.loadSnapshotResult, repo.loadSnapshotExists, repo.loadSnapshotErr
}

func (repo internalSnapshotChecksumRepo) SaveSnapshot(string, DirectoryChecksumSnapshot) error {
	return repo.saveSnapshotErr
}

type internalDaemonRepo struct {
	monitoredPaths map[string]bool
	readErr        error
}

func (r internalDaemonRepo) IsProjectMonitored(projectRoot string) (bool, error) {
	return r.monitoredPaths[projectRoot], r.readErr
}

func (r internalDaemonRepo) ProjectStatus(projectRoot string) (*DaemonProjectStatus, error) {
	return nil, r.readErr
}

func newValidInternalService(root string) InitCommandService {
	tree := newInternalProjectTree(root)
	tree.existsMap[indexFilePath(root)] = true

	return InitCommandService{
		projectTree:    tree,
		matcherFactory: internalMatcherFactory{matcher: internalMatcher{}},
		output:         internalOutput{},
		fileReader:     internalFileReader{files: map[string]string{}},
		indexer:        internalIndexer{},
		indexRepo:      internalIndexRepo{},
		checksumRepo:   internalChecksumRepo{loadData: map[string]string{}, exists: true},
		inspectUI:      internalInspectUIRunner{},
		initProgress:   disabledInitProgress{},
	}
}

func TestInitCommandService_ValidateDependencies_Branches(t *testing.T) {
	t.Parallel()

	// Arrange
	root := filepath.Join(string(filepath.Separator), "repo")
	base := newValidInternalService(root)

	cases := []struct {
		name   string
		mutate func(*InitCommandService)
	}{
		{name: "nil project tree", mutate: func(service *InitCommandService) { service.projectTree = nil }},
		{name: "nil matcher factory", mutate: func(service *InitCommandService) { service.matcherFactory = nil }},
		{name: "nil output", mutate: func(service *InitCommandService) { service.output = nil }},
		{name: "nil file reader", mutate: func(service *InitCommandService) { service.fileReader = nil }},
		{name: "nil indexer", mutate: func(service *InitCommandService) { service.indexer = nil }},
		{name: "nil index repo", mutate: func(service *InitCommandService) { service.indexRepo = nil }},
		{name: "nil checksum repo", mutate: func(service *InitCommandService) { service.checksumRepo = nil }},
		{name: "nil inspect UI", mutate: func(service *InitCommandService) { service.inspectUI = nil }},
	}

	for _, current := range cases {
		t.Run(current.name, func(t *testing.T) {
			t.Parallel()
			service := base
			current.mutate(&service)

			// Act / Assert
			require.Error(t, service.validateDependencies())
		})
	}

	// Act / Assert: all valid
	require.NoError(t, base.validateDependencies())
}

func TestInitCommandService_Sync_ReturnsSpecificErrorBranches(t *testing.T) {
	t.Parallel()

	root := filepath.Join(string(filepath.Separator), "repo")

	t.Run("find git root", func(t *testing.T) {
		t.Parallel()

		// Arrange
		service := newValidInternalService(root)
		service.projectTree.(*internalProjectTree).gitRootErr = errors.New("git root failure")

		// Act / Assert
		require.Error(t, service.Sync())
	})

	t.Run("root index exists check", func(t *testing.T) {
		t.Parallel()

		// Arrange
		service := newValidInternalService(root)
		service.projectTree.(*internalProjectTree).existsErr[indexFilePath(root)] = errors.New("exists failure")

		// Act / Assert
		require.Error(t, service.Sync())
	})

	t.Run("indexed directories read", func(t *testing.T) {
		t.Parallel()

		// Arrange
		service := newValidInternalService(root)
		service.projectTree.(*internalProjectTree).readDirErr[root] = errors.New("read dir failure")

		// Act / Assert
		require.Error(t, service.Sync())
	})

	t.Run("eligible directories matcher", func(t *testing.T) {
		t.Parallel()

		// Arrange
		service := newValidInternalService(root)
		service.matcherFactory = internalMatcherFactory{matcher: internalMatcher{err: errors.New("matcher failure")}}
		service.projectTree.(*internalProjectTree).readDirMap[root] = []filesystem.DirectoryEntry{{Name: "file.txt", Path: filepath.Join(root, "file.txt")}}

		// Act / Assert
		require.Error(t, service.Sync())
	})
}

func TestInitCommandService_InspectAndWriteInspectIndex_ErrorBranches(t *testing.T) {
	t.Parallel()

	// Arrange
	root := filepath.Join(string(filepath.Separator), "repo")
	service := newValidInternalService(root)
	tree := service.projectTree.(*internalProjectTree)

	// Exists error during Inspect
	tree.existsErr[indexFilePath(root)] = errors.New("exists failure")
	require.Error(t, service.Inspect("."))

	// Load error in LoadInspectIndex (single directory path)
	service.indexRepo = internalIndexRepo{loadErr: errors.New("load failure")}
	_, err := service.LoadInspectIndex(".")
	require.Error(t, err)

	// JSON marshal error for NaN in writeIndexJSON
	invalid := NewInvertedIndex()
	invalid.AverageDocLength = math.NaN()
	require.Error(t, service.writeIndexJSON(invalid))
}

func TestInitCommandService_StorageHelper_ErrorBranches(t *testing.T) {
	t.Parallel()

	// Arrange
	root := filepath.Join(string(filepath.Separator), "repo")
	service := newValidInternalService(root)
	tree := service.projectTree.(*internalProjectTree)

	// Exists error in hasDirectoryIndex
	tree.existsErr[indexFilePath(root)] = errors.New("exists failure")
	_, err := service.hasDirectoryIndex(root)
	require.Error(t, err)

	// Checksum load error
	service.checksumRepo = internalChecksumRepo{loadErr: errors.New("load failure")}
	_, _, err = service.loadChecksumSnapshot(root)
	require.Error(t, err)
}

// keep assert imported via the table-driven test to avoid unused-import errors.
var _ = assert.NoError
