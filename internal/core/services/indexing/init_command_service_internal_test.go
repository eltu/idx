package indexing

import (
	"errors"
	"math"
	"path/filepath"
	"testing"

	"idx/internal/core/domain"
	"idx/internal/core/ports"
)

type internalProjectTree struct {
	currentDir string
	currentErr error
	gitRoot    string
	gitRootErr error
	readDirMap map[string][]domain.DirectoryEntry
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
		readDirMap: map[string][]domain.DirectoryEntry{},
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

func (tree *internalProjectTree) ReadDir(path string) ([]domain.DirectoryEntry, error) {
	if err, ok := tree.readDirErr[path]; ok {
		return nil, err
	}

	entries, ok := tree.readDirMap[path]
	if !ok {
		return []domain.DirectoryEntry{}, nil
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
	matcher ports.IgnoreMatcher
	err     error
}

func (factory internalMatcherFactory) New(_ string) (ports.IgnoreMatcher, error) {
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

func (internalInspectUIRunner) Run(_ *domain.InvertedIndex) error { return nil }

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

func (indexer internalIndexer) BuildIndex(_ []domain.IndexDocument) (*domain.InvertedIndex, error) {
	if indexer.err != nil {
		return nil, indexer.err
	}

	return domain.NewInvertedIndex(), nil
}

type internalIndexRepo struct {
	loadErr error
	saveErr error
	index   *domain.InvertedIndex
}

func (repo internalIndexRepo) LoadIndex(_ string) (*domain.InvertedIndex, error) {
	if repo.loadErr != nil {
		return nil, repo.loadErr
	}

	if repo.index != nil {
		return repo.index, nil
	}

	return domain.NewInvertedIndex(), nil
}

func (repo internalIndexRepo) SaveIndex(_ string, _ *domain.InvertedIndex) error {
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
	loadSnapshotResult ports.DirectoryChecksumSnapshot
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

func (repo internalSnapshotChecksumRepo) LoadSnapshot(string) (ports.DirectoryChecksumSnapshot, bool, error) {
	return repo.loadSnapshotResult, repo.loadSnapshotExists, repo.loadSnapshotErr
}

func (repo internalSnapshotChecksumRepo) SaveSnapshot(string, ports.DirectoryChecksumSnapshot) error {
	return repo.saveSnapshotErr
}

type internalDaemonRepo struct {
	state   *domain.DaemonState
	readErr error
}

func (r internalDaemonRepo) ReadState() (*domain.DaemonState, error) {
	return r.state, r.readErr
}

func (r internalDaemonRepo) SaveState(_ *domain.DaemonState) error { return nil }

func (r internalDaemonRepo) UpdateProjectPID(_ string, _ int) error { return nil }

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
	}
}

func TestInitCommandServiceValidateDependenciesBranches(t *testing.T) {
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
			service := base
			current.mutate(&service)

			if err := service.validateDependencies(); err == nil {
				t.Fatal("expected dependency validation error")
			}
		})
	}

	if err := base.validateDependencies(); err != nil {
		t.Fatalf("expected valid dependencies, got %v", err)
	}
}

func TestInitCommandServiceSyncReturnsSpecificErrorBranches(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")

	t.Run("find git root", func(t *testing.T) {
		service := newValidInternalService(root)
		service.projectTree.(*internalProjectTree).gitRootErr = errors.New("git root failure")

		if err := service.Sync(); err == nil {
			t.Fatal("expected find git root error")
		}
	})

	t.Run("root index exists check", func(t *testing.T) {
		service := newValidInternalService(root)
		service.projectTree.(*internalProjectTree).existsErr[indexFilePath(root)] = errors.New("exists failure")

		if err := service.Sync(); err == nil {
			t.Fatal("expected exists error")
		}
	})

	t.Run("indexed directories read", func(t *testing.T) {
		service := newValidInternalService(root)
		service.projectTree.(*internalProjectTree).readDirErr[root] = errors.New("read dir failure")

		if err := service.Sync(); err == nil {
			t.Fatal("expected indexed directories error")
		}
	})

	t.Run("eligible directories matcher", func(t *testing.T) {
		service := newValidInternalService(root)
		service.matcherFactory = internalMatcherFactory{matcher: internalMatcher{err: errors.New("matcher failure")}}
		service.projectTree.(*internalProjectTree).readDirMap[root] = []domain.DirectoryEntry{{Name: "file.txt", Path: filepath.Join(root, "file.txt")}}

		if err := service.Sync(); err == nil {
			t.Fatal("expected matcher error while selecting eligible directories")
		}
	})
}

func TestInitCommandServiceInspectAndWriteInspectIndexErrorBranches(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
	service := newValidInternalService(root)
	tree := service.projectTree.(*internalProjectTree)

	tree.existsErr[indexFilePath(root)] = errors.New("exists failure")
	if err := service.Inspect("."); err == nil {
		t.Fatal("expected inspect exists error")
	}

	service.indexRepo = internalIndexRepo{loadErr: errors.New("load failure")}
	if err := service.writeInspectIndex(root); err == nil {
		t.Fatal("expected inspect load error")
	}

	invalid := domain.NewInvertedIndex()
	invalid.AverageDocLength = math.NaN()
	service.indexRepo = internalIndexRepo{index: invalid}
	if err := service.writeInspectIndex(root); err == nil {
		t.Fatal("expected JSON marshal error for invalid NaN payload")
	}
}

func TestInitCommandServiceStorageHelperErrorBranches(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
	service := newValidInternalService(root)
	tree := service.projectTree.(*internalProjectTree)

	tree.existsErr[indexFilePath(root)] = errors.New("exists failure")
	if _, err := service.hasDirectoryIndex(root); err == nil {
		t.Fatal("expected hasDirectoryIndex error")
	}

	service.checksumRepo = internalChecksumRepo{loadErr: errors.New("load failure")}
	if _, _, err := service.loadChecksumSnapshot(root); err == nil {
		t.Fatal("expected checksum load error")
	}
}
