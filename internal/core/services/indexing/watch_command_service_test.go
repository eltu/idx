package indexing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	"idx/internal/core/domain"
	"idx/internal/core/ports"
)

func TestShouldTrackFileEventCreate(t *testing.T) {
	if !shouldTrackFileEvent(fsnotify.Create) {
		t.Fatal("expected Create to be tracked")
	}
}

func TestShouldTrackFileEventWrite(t *testing.T) {
	if !shouldTrackFileEvent(fsnotify.Write) {
		t.Fatal("expected Write to be tracked")
	}
}

func TestShouldTrackFileEventRename(t *testing.T) {
	if !shouldTrackFileEvent(fsnotify.Rename) {
		t.Fatal("expected Rename to be tracked")
	}
}

func TestShouldTrackFileEventRemove(t *testing.T) {
	if !shouldTrackFileEvent(fsnotify.Remove) {
		t.Fatal("expected Remove to be tracked")
	}
}

func TestShouldTrackFileEventChmodIgnored(t *testing.T) {
	if shouldTrackFileEvent(fsnotify.Chmod) {
		t.Fatal("expected Chmod to not be tracked")
	}
}

func TestHasSystemPathSegmentDetectsGit(t *testing.T) {
	if !hasSystemPathSegment("/repo/.git/refs/HEAD") {
		t.Fatal("expected .git path segment to be detected")
	}
}

func TestHasSystemPathSegmentDetectsIdx(t *testing.T) {
	if !hasSystemPathSegment("/repo/.idx/index.gob") {
		t.Fatal("expected .idx path segment to be detected")
	}
}

func TestHasSystemPathSegmentReturnsFalseForNormal(t *testing.T) {
	if hasSystemPathSegment("/repo/internal/core/service.go") {
		t.Fatal("expected normal path to not be detected as system")
	}
}

func TestIsWithinRootReturnsTrueForChild(t *testing.T) {
	if !isWithinRoot("/repo", "/repo/internal/core") {
		t.Fatal("expected child path to be within root")
	}
}

func TestIsWithinRootReturnsTrueForSelf(t *testing.T) {
	if !isWithinRoot("/repo", "/repo") {
		t.Fatal("expected root itself to be within root")
	}
}

func TestIsWithinRootReturnsFalseForSibling(t *testing.T) {
	if isWithinRoot("/repo", "/other") {
		t.Fatal("expected sibling path to not be within root")
	}
}

func TestIsWithinRootReturnsFalseForParent(t *testing.T) {
	if isWithinRoot("/repo/internal", "/repo") {
		t.Fatal("expected parent path to not be within child root")
	}
}

func TestShouldSkipSystemDirectoryGit(t *testing.T) {
	if !shouldSkipSystemDirectory("/repo/.git") {
		t.Fatal("expected .git directory to be skipped")
	}
}

func TestShouldSkipSystemDirectoryIdx(t *testing.T) {
	if !shouldSkipSystemDirectory("/repo/.idx") {
		t.Fatal("expected .idx directory to be skipped")
	}
}

func TestShouldSkipSystemDirectoryNormalDir(t *testing.T) {
	if shouldSkipSystemDirectory("/repo/internal") {
		t.Fatal("expected normal directory to not be skipped")
	}
}

func TestSortedDirectoryBatchReturnsSorted(t *testing.T) {
	pending := map[string]struct{}{
		"/repo/z": {},
		"/repo/a": {},
		"/repo/m": {},
	}

	result := sortedDirectoryBatch(pending)
	if len(result) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(result))
	}

	if result[0] != "/repo/a" || result[1] != "/repo/m" || result[2] != "/repo/z" {
		t.Fatalf("expected sorted order, got %v", result)
	}
}

func TestSortedDirectoryBatchEmptyReturnsEmpty(t *testing.T) {
	result := sortedDirectoryBatch(map[string]struct{}{})
	if len(result) != 0 {
		t.Fatalf("expected empty result, got %v", result)
	}
}

func TestSortedFileBatchReturnsSorted(t *testing.T) {
	pending := map[string]struct{}{
		"cmd/main.go":      {},
		"internal/core.go": {},
		"a/b.go":           {},
	}

	result := sortedFileBatch(pending)
	if len(result) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(result))
	}

	if result[0] != "a/b.go" {
		t.Fatalf("expected 'a/b.go' first, got %q", result[0])
	}
}

func TestSortedFileBatchEmptyReturnsEmpty(t *testing.T) {
	result := sortedFileBatch(map[string]struct{}{})
	if len(result) != 0 {
		t.Fatalf("expected empty result, got %v", result)
	}
}

func TestResetDebounceTimerCreatesNewTimerWhenNil(t *testing.T) {
	debounce := 50 * time.Millisecond
	timer, ch := resetDebounceTimer(nil, debounce)
	if timer == nil {
		t.Fatal("expected non-nil timer")
	}
	if ch == nil {
		t.Fatal("expected non-nil channel")
	}
	timer.Stop()
}

func TestResetDebounceTimerResetsExistingTimer(t *testing.T) {
	debounce := 50 * time.Millisecond
	firstTimer, _ := resetDebounceTimer(nil, debounce)
	secondTimer, ch := resetDebounceTimer(firstTimer, debounce)
	if secondTimer == nil || ch == nil {
		t.Fatal("expected non-nil timer and channel after reset")
	}
	secondTimer.Stop()
}

func TestIsIgnoredPathReturnsFalseForRoot(t *testing.T) {
	root := t.TempDir()
	ignored, err := isIgnoredPath(root, root, true, neverMatcher{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ignored {
		t.Fatal("expected root path to not be ignored")
	}
}

func TestIsIgnoredPathDelegatesToMatcher(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "vendor")
	ignored, err := isIgnoredPath(root, child, true, alwaysMatcher{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !ignored {
		t.Fatal("expected child path to be ignored by alwaysMatcher")
	}
}

func TestEventDirectoryForExistingFile(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file.go")
	if err := os.WriteFile(file, []byte("package main"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	dir, ok := eventDirectory(root, file)
	if !ok {
		t.Fatal("expected eventDirectory to succeed for existing file")
	}
	if dir != root {
		t.Fatalf("expected directory %q, got %q", root, dir)
	}
}

func TestEventDirectoryForExistingDir(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "subdir")
	if err := os.Mkdir(child, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	dir, ok := eventDirectory(root, child)
	if !ok {
		t.Fatal("expected eventDirectory to succeed for existing directory")
	}
	if dir != child {
		t.Fatalf("expected directory %q, got %q", child, dir)
	}
}

func TestEventDirectoryOutsideRootReturnsFalse(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "outside")

	_, ok := eventDirectory(root, outside)
	if ok {
		t.Fatal("expected eventDirectory to return false for path outside root")
	}
}

// neverMatcher is a ports.IgnoreMatcher that never matches.
type neverMatcher struct{}

func (neverMatcher) Matches(_ string) (bool, error) { return false, nil }

// alwaysMatcher is a ports.IgnoreMatcher that always matches.
type alwaysMatcher struct{}

func (alwaysMatcher) Matches(_ string) (bool, error) { return true, nil }

var _ ports.IgnoreMatcher = neverMatcher{}
var _ ports.IgnoreMatcher = alwaysMatcher{}

func newWatchService(root string) InitCommandService {
	tree := &internalWatchProjectTree{root: root}
	return InitCommandService{
		projectTree:    tree,
		matcherFactory: internalWatchMatcherFactory{},
		output:         &internalWatchOutput{},
		fileReader:     internalWatchFileReader{},
		indexer:        internalWatchIndexer{},
		indexRepo:      internalWatchIndexRepo{},
		checksumRepo:   internalWatchChecksumRepo{},
		inspectUI:      internalWatchInspectUI{},
	}
}

// internalWatchProjectTree implements ports.ProjectTree for watch tests.
type internalWatchProjectTree struct{ root string }

func (t *internalWatchProjectTree) CurrentDir() (string, error)          { return t.root, nil }
func (t *internalWatchProjectTree) FindGitRoot(_ string) (string, error) { return t.root, nil }
func (t *internalWatchProjectTree) ReadDir(_ string) ([]domain.DirectoryEntry, error) {
	return nil, nil
}
func (t *internalWatchProjectTree) Exists(_ string) (bool, error)      { return false, nil }
func (t *internalWatchProjectTree) WriteFile(_ string, _ []byte) error { return nil }
func (t *internalWatchProjectTree) RemoveAll(_ string) error           { return nil }

// internalWatchMatcherFactory returns a matcher that never ignores.
type internalWatchMatcherFactory struct{}

func (internalWatchMatcherFactory) New(_ string) (ports.IgnoreMatcher, error) {
	return neverMatcher{}, nil
}

// Simple no-op implementations for remaining watch service deps.
type internalWatchOutput struct{ lines []string }

func (o *internalWatchOutput) WriteLine(line string) error {
	o.lines = append(o.lines, line)
	return nil
}

type internalWatchFileReader struct{}

func (internalWatchFileReader) ReadFile(_ string) (string, error) { return "", nil }

type internalWatchIndexer struct{}

func (internalWatchIndexer) BuildIndex(_ []domain.IndexDocument) (*domain.InvertedIndex, error) {
	return domain.NewInvertedIndex(), nil
}

type internalWatchIndexRepo struct{}

func (internalWatchIndexRepo) LoadIndex(_ string) (*domain.InvertedIndex, error) {
	return domain.NewInvertedIndex(), nil
}
func (internalWatchIndexRepo) SaveIndex(_ string, _ *domain.InvertedIndex) error { return nil }

type internalWatchChecksumRepo struct{}

func (internalWatchChecksumRepo) Load(_ string) (map[string]string, bool, error) {
	return nil, false, nil
}
func (internalWatchChecksumRepo) Save(_ string, _ map[string]string) error { return nil }
func (internalWatchChecksumRepo) LoadSnapshot(_ string) (ports.DirectoryChecksumSnapshot, bool, error) {
	return ports.DirectoryChecksumSnapshot{}, false, nil
}
func (internalWatchChecksumRepo) SaveSnapshot(_ string, _ ports.DirectoryChecksumSnapshot) error {
	return nil
}

type internalWatchInspectUI struct{}

func (internalWatchInspectUI) Run(_ *domain.InvertedIndex) error { return nil }

func TestTrackEventDirectoriesAddsPathForTrackedEvent(t *testing.T) {
	root := t.TempDir()
	subdir := filepath.Join(root, "pkg")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	service := newWatchService(root)
	pending := make(map[string]struct{})

	service.trackEventDirectories(
		fsnotify.Event{Op: fsnotify.Write, Name: filepath.Join(subdir, "file.go")},
		root, neverMatcher{}, pending,
	)

	if _, ok := pending[subdir]; !ok {
		t.Fatal("expected subdir to be added to pending directories")
	}
}

func TestTrackEventDirectoriesSkipsNonTrackedOp(t *testing.T) {
	root := t.TempDir()
	service := newWatchService(root)
	pending := make(map[string]struct{})

	service.trackEventDirectories(
		fsnotify.Event{Op: fsnotify.Chmod, Name: filepath.Join(root, "file.go")},
		root, neverMatcher{}, pending,
	)

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

	service.trackEventFiles(
		fsnotify.Event{Op: fsnotify.Write, Name: file},
		root, neverMatcher{}, pending,
	)

	if _, ok := pending["main.go"]; !ok {
		t.Fatalf("expected main.go in pending files, got %v", pending)
	}
}

func TestTrackEventFilesSkipsNonTrackedOp(t *testing.T) {
	root := t.TempDir()
	service := newWatchService(root)
	pending := make(map[string]struct{})

	service.trackEventFiles(
		fsnotify.Event{Op: fsnotify.Chmod, Name: filepath.Join(root, "file.go")},
		root, neverMatcher{}, pending,
	)

	if len(pending) != 0 {
		t.Fatal("expected no entries for non-tracked op")
	}
}

func TestWriteUpdatedFilesWithFiles(t *testing.T) {
	out := &internalWatchOutput{}
	service := newWatchService(t.TempDir())
	service.output = out

	pending := map[string]struct{}{
		"internal/service.go": {},
		"cmd/main.go":         {},
	}

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

	err := service.flushWatchedBatch(
		map[string]struct{}{}, // no pending directories
		map[string]struct{}{},
		t.TempDir(),
		neverMatcher{},
		false,
	)
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

	// internalWatchProjectTree.Exists returns false, so ensureRootIndex will create it.
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

	// Use a tree that reports the index as existing.
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

// internalWatchExistsProjectTree reports all paths as existing.
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
	tree.readDirMap[root] = []domain.DirectoryEntry{
		{Name: "src", Path: sourceDir, IsDir: true},
		{Name: "vendor", Path: ignoredDir, IsDir: true},
		{Name: "main.go", Path: rootFile, IsDir: false, Size: int64(len("package main")), ModTimeUnixNano: 1},
	}
	tree.readDirMap[sourceDir] = []domain.DirectoryEntry{
		{Name: "service.go", Path: sourceFile, IsDir: false, Size: int64(len("package service")), ModTimeUnixNano: 2},
	}
	tree.readDirMap[ignoredDir] = []domain.DirectoryEntry{
		{Name: "legacy.go", Path: filepath.Join(ignoredDir, "legacy.go"), IsDir: false},
	}

	matcher := watchStartupMatcher{ignoredPrefixes: []string{"vendor"}}
	indexRepo := &watchStartupIndexRepo{}
	checksumRepo := &watchStartupChecksumRepo{}

	service := InitCommandService{
		projectTree:    tree,
		matcherFactory: internalMatcherFactory{matcher: matcher},
		output:         internalOutput{},
		fileReader: internalFileReader{files: map[string]string{
			rootFile:   "package main",
			sourceFile: "package service",
		}},
		indexer:      internalIndexer{},
		indexRepo:    indexRepo,
		checksumRepo: checksumRepo,
		inspectUI:    internalInspectUIRunner{},
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

type watchStartupMatcher struct {
	ignoredPrefixes []string
}

func (matcher watchStartupMatcher) Matches(path string) (bool, error) {
	for _, prefix := range matcher.ignoredPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true, nil
		}
	}

	return false, nil
}

type watchStartupIndexRepo struct {
	savedDirectories []string
}

func (repo *watchStartupIndexRepo) SaveIndex(directoryPath string, _ *domain.InvertedIndex) error {
	repo.savedDirectories = append(repo.savedDirectories, directoryPath)
	return nil
}

func (repo *watchStartupIndexRepo) LoadIndex(_ string) (*domain.InvertedIndex, error) {
	return domain.NewInvertedIndex(), nil
}

type watchStartupChecksumRepo struct {
	loadData map[string]map[string]string
}

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
