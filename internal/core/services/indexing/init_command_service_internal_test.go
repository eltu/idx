package indexing

import (
	"errors"
	"math"
	"path/filepath"
	"testing"
	"time"

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

type internalOutput struct{}

func (internalOutput) WriteLine(string) error { return nil }

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

func TestInitCommandServiceBuildAndSaveIndexErrorBranches(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
	entry := domain.DirectoryEntry{Name: "a.txt", Path: filepath.Join(root, "a.txt")}
	snapshot := ports.DirectoryChecksumSnapshot{Files: map[string]ports.FileChecksumState{}}

	service := newValidInternalService(root)
	service.fileReader = internalFileReader{err: errors.New("read failure")}
	if err := service.buildAndSaveIndex(root, []domain.DirectoryEntry{entry}, snapshot, map[string]struct{}{}); err == nil {
		t.Fatal("expected file read error")
	}

	service = newValidInternalService(root)
	service.fileReader = internalFileReader{files: map[string]string{entry.Path: "hello"}}
	service.indexer = internalIndexer{err: errors.New("indexer failure")}
	if err := service.buildAndSaveIndex(root, []domain.DirectoryEntry{entry}, snapshot, map[string]struct{}{}); err == nil {
		t.Fatal("expected indexer error")
	}

	service = newValidInternalService(root)
	service.fileReader = internalFileReader{files: map[string]string{entry.Path: "hello"}}
	service.indexRepo = internalIndexRepo{saveErr: errors.New("save failure")}
	if err := service.buildAndSaveIndex(root, []domain.DirectoryEntry{entry}, snapshot, map[string]struct{}{}); err == nil {
		t.Fatal("expected index save error")
	}
}

func TestInitCommandServiceShouldReindexDirectoryBranches(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
	service := newValidInternalService(root)
	tree := service.projectTree.(*internalProjectTree)

	tree.existsErr[indexFilePath(root)] = errors.New("exists failure")
	if _, err := service.shouldReindexDirectory(root, map[string]string{}); err == nil {
		t.Fatal("expected hasDirectoryIndex error")
	}

	service = newValidInternalService(root)
	service.checksumRepo = internalChecksumRepo{loadErr: errors.New("load failure")}
	if _, err := service.shouldReindexDirectory(root, map[string]string{}); err == nil {
		t.Fatal("expected checksum repository load error")
	}

	service = newValidInternalService(root)
	service.checksumRepo = internalChecksumRepo{loadData: map[string]string{}, exists: false}
	should, err := service.shouldReindexDirectory(root, map[string]string{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !should {
		t.Fatal("expected reindex when checksums are not present")
	}
}

func TestInitCommandServiceHelpersCoverUncoveredBranches(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
	indexedAt := time.Unix(1700000000, 0).UTC()

	if got := buildChangedLogEntries(nil, ports.DirectoryChecksumSnapshot{Files: map[string]ports.FileChecksumState{}}, map[string]struct{}{}, indexedAt); len(got) != 0 {
		t.Fatalf("expected empty log entries, got %d", len(got))
	}

	entries := []domain.DirectoryEntry{{Name: "a.txt", Path: filepath.Join(root, "a.txt")}}
	snapshot := ports.DirectoryChecksumSnapshot{Files: map[string]ports.FileChecksumState{}}
	changed := map[string]struct{}{"a.txt": {}}
	if got := buildChangedLogEntries(entries, snapshot, changed, indexedAt); len(got) != 0 {
		t.Fatalf("expected skip when snapshot state missing, got %d", len(got))
	}

	service := newValidInternalService(root)
	children := []domain.DirectoryEntry{{Name: "not-dir", Path: filepath.Join(root, "f.txt"), IsDir: false}}
	if err := service.indexChildren(children, root, internalMatcher{}); err != nil {
		t.Fatalf("expected indexChildren to skip non-directories, got %v", err)
	}

	_, err := filterEntries([]domain.DirectoryEntry{{Name: "a.txt", Path: filepath.Join(root, "a.txt")}}, root, internalMatcher{err: errors.New("matcher failure")})
	if err == nil {
		t.Fatal("expected filterEntries matcher error")
	}

	if sameChecksums(map[string]string{"a": "1"}, map[string]string{"a": "1", "b": "2"}) {
		t.Fatal("expected sameChecksums false when map sizes differ")
	}

	if sameSnapshotChecksums(
		map[string]ports.FileChecksumState{"a": {Checksum: "1"}},
		map[string]ports.FileChecksumState{"a": {Checksum: "2"}},
	) {
		t.Fatal("expected sameSnapshotChecksums false when checksum differs")
	}
}

func TestInitCommandServiceDirectoryChecksumsAndSameChecksumsBranches(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
	service := newValidInternalService(root)

	firstPath := filepath.Join(root, "a.txt")
	secondPath := filepath.Join(root, "b.txt")
	service.fileReader = internalFileReader{files: map[string]string{firstPath: "a", secondPath: "b"}}

	checksums, err := service.directoryChecksums([]domain.DirectoryEntry{
		{Name: "a.txt", Path: firstPath},
		{Name: "b.txt", Path: secondPath},
	})
	if err != nil {
		t.Fatalf("expected checksums without error, got %v", err)
	}
	if len(checksums) != 2 {
		t.Fatalf("expected 2 checksums, got %d", len(checksums))
	}

	service.fileReader = internalFileReader{err: errors.New("read failure")}
	if _, err := service.directoryChecksums([]domain.DirectoryEntry{{Name: "a.txt", Path: firstPath}}); err == nil {
		t.Fatal("expected directoryChecksums read error")
	}

	if !sameChecksums(map[string]string{"a": "1"}, map[string]string{"a": "1"}) {
		t.Fatal("expected sameChecksums true for equal maps")
	}
	if sameChecksums(map[string]string{"a": "1"}, map[string]string{"b": "1"}) {
		t.Fatal("expected sameChecksums false when key is missing")
	}
	if sameChecksums(map[string]string{"a": "1"}, map[string]string{"a": "2"}) {
		t.Fatal("expected sameChecksums false when value differs")
	}
}

func TestInitCommandServiceLoadAndSaveChecksumSnapshotBranches(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
	service := newValidInternalService(root)

	service.checksumRepo = internalChecksumRepo{loadData: map[string]string{"a.txt": "hash"}, exists: true}
	snapshot, exists, err := service.loadChecksumSnapshot(root)
	if err != nil {
		t.Fatalf("expected snapshot load without error, got %v", err)
	}
	if !exists || snapshot.Files["a.txt"].Checksum != "hash" {
		t.Fatalf("expected converted snapshot state, got exists=%v snapshot=%v", exists, snapshot.Files)
	}

	service.checksumRepo = internalChecksumRepo{saveErr: errors.New("save failure")}
	err = service.saveChecksumSnapshot(root, ports.DirectoryChecksumSnapshot{Files: map[string]ports.FileChecksumState{"a.txt": {Checksum: "hash"}}})
	if err == nil {
		t.Fatal("expected fallback save checksum error")
	}

	service.checksumRepo = internalSnapshotChecksumRepo{
		loadSnapshotResult: ports.DirectoryChecksumSnapshot{Files: map[string]ports.FileChecksumState{"a.txt": {Checksum: "hash"}}},
		loadSnapshotExists: true,
	}
	snapshot, exists, err = service.loadChecksumSnapshot(root)
	if err != nil {
		t.Fatalf("expected snapshot repository load without error, got %v", err)
	}
	if !exists || snapshot.Files["a.txt"].Checksum != "hash" {
		t.Fatalf("expected snapshot repository data, got exists=%v snapshot=%v", exists, snapshot.Files)
	}

	service.checksumRepo = internalSnapshotChecksumRepo{saveSnapshotErr: errors.New("snapshot save failure")}
	err = service.saveChecksumSnapshot(root, ports.DirectoryChecksumSnapshot{Files: map[string]ports.FileChecksumState{}})
	if err == nil {
		t.Fatal("expected snapshot repository save error")
	}
}

func TestInitCommandServiceShouldReindexDirectoryAndMetadataBranches(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
	service := newValidInternalService(root)
	tree := service.projectTree.(*internalProjectTree)

	tree.existsMap[indexFilePath(root)] = false
	should, err := service.shouldReindexDirectory(root, map[string]string{})
	if err != nil {
		t.Fatalf("expected no error when no index, got %v", err)
	}
	if !should {
		t.Fatal("expected shouldReindexDirectory true when index is missing")
	}

	tree.existsMap[indexFilePath(root)] = true
	service.checksumRepo = internalChecksumRepo{loadData: map[string]string{"a": "1"}, exists: true}
	should, err = service.shouldReindexDirectory(root, map[string]string{"a": "1"})
	if err != nil {
		t.Fatalf("expected no error for equal checksums, got %v", err)
	}
	if should {
		t.Fatal("expected shouldReindexDirectory false when checksums are equal")
	}

	should, err = service.shouldReindexDirectory(root, map[string]string{"a": "2"})
	if err != nil {
		t.Fatalf("expected no error for different checksums, got %v", err)
	}
	if !should {
		t.Fatal("expected shouldReindexDirectory true when checksums differ")
	}

	if metadataUnchanged(domain.DirectoryEntry{Size: 10, ModTimeUnixNano: 20}, ports.FileChecksumState{Size: 10, ModTimeUnixNano: 20}) == false {
		t.Fatal("expected metadataUnchanged true for identical metadata")
	}
}

func TestInitCommandServiceRemoveDirectoryIndexErrorBranch(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
	service := newValidInternalService(root)
	service.projectTree.(*internalProjectTree).removeErr = errors.New("remove failure")

	err := service.removeDirectoryIndex(root)
	if err == nil {
		t.Fatal("expected removeDirectoryIndex error")
	}
}
