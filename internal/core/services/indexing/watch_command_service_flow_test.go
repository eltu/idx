package indexing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fsnotify/fsnotify"

	"idx/internal/core/domain"
)

func TestTrackEventDirectoriesAddsPathForTrackedEvent(t *testing.T) {
	root := t.TempDir()
	subdir := filepath.Join(root, "pkg")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	service := newWatchService(root)
	pending := make(map[string]struct{})
	service.trackEventDirectories(fsnotify.Event{Op: fsnotify.Write, Name: filepath.Join(subdir, "file.go")}, root, neverMatcher{}, pending)

	if _, ok := pending[subdir]; !ok {
		t.Fatal("expected subdir to be added to pending directories")
	}
}

func TestTrackEventDirectoriesSkipsNonTrackedOp(t *testing.T) {
	root := t.TempDir()
	service := newWatchService(root)
	pending := make(map[string]struct{})

	service.trackEventDirectories(fsnotify.Event{Op: fsnotify.Chmod, Name: filepath.Join(root, "file.go")}, root, neverMatcher{}, pending)
	if len(pending) != 0 {
		t.Fatal("expected no entries for non-tracked op")
	}
}

func TestTrackEventFilesAddsRelativePathForExistingFile(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "main.go")
	if err := os.WriteFile(file, []byte("package main"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	service := newWatchService(root)
	pending := make(map[string]struct{})
	service.trackEventFiles(fsnotify.Event{Op: fsnotify.Write, Name: file}, root, neverMatcher{}, pending)

	if _, ok := pending["main.go"]; !ok {
		t.Fatalf("expected main.go in pending files, got %v", pending)
	}
}

func TestTrackEventFilesSkipsNonTrackedOp(t *testing.T) {
	root := t.TempDir()
	service := newWatchService(root)
	pending := make(map[string]struct{})

	service.trackEventFiles(fsnotify.Event{Op: fsnotify.Chmod, Name: filepath.Join(root, "file.go")}, root, neverMatcher{}, pending)
	if len(pending) != 0 {
		t.Fatal("expected no entries for non-tracked op")
	}
}

func TestWriteUpdatedFilesWithFiles(t *testing.T) {
	out := &internalWatchOutput{}
	service := newWatchService(t.TempDir())
	service.output = out

	pending := map[string]struct{}{"internal/service.go": {}, "cmd/main.go": {}}
	if err := service.writeUpdatedFiles(pending); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(out.lines) == 0 {
		t.Fatal("expected output lines from writeUpdatedFiles")
	}
}

func TestWriteUpdatedFilesWithEmpty(t *testing.T) {
	out := &internalWatchOutput{}
	service := newWatchService(t.TempDir())
	service.output = out

	if err := service.writeUpdatedFiles(map[string]struct{}{}); err != nil {
		t.Fatalf("expected no error for empty pending files, got %v", err)
	}
	if len(out.lines) != 1 || out.lines[0] != "   files: none" {
		t.Fatalf("expected 'files: none' output, got %v", out.lines)
	}
}

func TestFlushWatchedBatchWithEmptyDirectoriesIsNoop(t *testing.T) {
	out := &internalWatchOutput{}
	service := newWatchService(t.TempDir())
	service.output = out

	err := service.flushWatchedBatch(map[string]struct{}{}, map[string]struct{}{}, t.TempDir(), neverMatcher{}, false)
	if err != nil {
		t.Fatalf("expected no error for empty batch, got %v", err)
	}
	if len(out.lines) != 0 {
		t.Fatalf("expected no output for empty batch, got %v", out.lines)
	}
}

func TestEnsureRootIndexCreatesIndexWhenAbsent(t *testing.T) {
	root := t.TempDir()
	out := &internalWatchOutput{}
	service := newWatchService(root)
	service.output = out

	if err := service.ensureRootIndex(root, neverMatcher{}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(out.lines) == 0 {
		t.Fatal("expected output lines when root index is created")
	}
}

func TestEnsureRootIndexSkipsWhenIndexExists(t *testing.T) {
	root := t.TempDir()
	out := &internalWatchOutput{}

	tree := &internalWatchExistsProjectTree{root: root}
	service := newWatchService(root)
	service.projectTree = tree
	service.output = out

	if err := service.ensureRootIndex(root, neverMatcher{}); err != nil {
		t.Fatalf("expected no error when index already exists, got %v", err)
	}
	if len(out.lines) != 0 {
		t.Fatalf("expected no output when index already exists, got %v", out.lines)
	}
}

type internalWatchExistsProjectTree struct{ root string }

func (t *internalWatchExistsProjectTree) CurrentDir() (string, error)          { return t.root, nil }
func (t *internalWatchExistsProjectTree) FindGitRoot(_ string) (string, error) { return t.root, nil }
func (t *internalWatchExistsProjectTree) ReadDir(_ string) ([]domain.DirectoryEntry, error) {
	return nil, nil
}
func (t *internalWatchExistsProjectTree) Exists(_ string) (bool, error)      { return true, nil }
func (t *internalWatchExistsProjectTree) WriteFile(_ string, _ []byte) error { return nil }
func (t *internalWatchExistsProjectTree) RemoveAll(_ string) error           { return nil }

func TestSyncAllDirectoriesBeforeWatchSyncsEligibleAndRemovesStale(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "repo")
	sourceDir := filepath.Join(root, "src")
	ignoredDir := filepath.Join(root, "vendor")
	rootFile := filepath.Join(root, "main.go")
	sourceFile := filepath.Join(sourceDir, "service.go")

	tree := newInternalProjectTree(root)
	tree.existsMap[indexFilePath(root)] = true
	tree.existsMap[indexFilePath(sourceDir)] = false
	tree.existsMap[indexFilePath(ignoredDir)] = true
	tree.readDirMap[root] = []domain.DirectoryEntry{{Name: "src", Path: sourceDir, IsDir: true}, {Name: "vendor", Path: ignoredDir, IsDir: true}, {Name: "main.go", Path: rootFile, IsDir: false, Size: int64(len("package main")), ModTimeUnixNano: 1}}
	tree.readDirMap[sourceDir] = []domain.DirectoryEntry{{Name: "service.go", Path: sourceFile, IsDir: false, Size: int64(len("package service")), ModTimeUnixNano: 2}}
	tree.readDirMap[ignoredDir] = []domain.DirectoryEntry{{Name: "legacy.go", Path: filepath.Join(ignoredDir, "legacy.go"), IsDir: false}}

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
	}

	if err := service.syncAllDirectoriesBeforeWatch(root, matcher); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !containsPath(tree.removed, filepath.Join(ignoredDir, ".idx")) {
		t.Fatalf("expected stale ignored directory index to be removed, removed=%v", tree.removed)
	}
	if !containsPath(indexRepo.savedDirectories, root) {
		t.Fatalf("expected root directory to be synchronized, got %v", indexRepo.savedDirectories)
	}
	if !containsPath(indexRepo.savedDirectories, sourceDir) {
		t.Fatalf("expected source directory to be synchronized, got %v", indexRepo.savedDirectories)
	}
	if containsPath(indexRepo.savedDirectories, ignoredDir) {
		t.Fatalf("expected ignored directory to be skipped, got %v", indexRepo.savedDirectories)
	}
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

type watchStartupIndexRepo struct{ savedDirectories []string }

func (repo *watchStartupIndexRepo) SaveIndex(directoryPath string, _ *domain.InvertedIndex) error {
	repo.savedDirectories = append(repo.savedDirectories, directoryPath)
	return nil
}

func (repo *watchStartupIndexRepo) LoadIndex(_ string) (*domain.InvertedIndex, error) {
	return domain.NewInvertedIndex(), nil
}

type watchStartupChecksumRepo struct{ loadData map[string]map[string]string }

func (repo *watchStartupChecksumRepo) Load(directoryPath string) (map[string]string, bool, error) {
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
