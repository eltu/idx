package indexing

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	"idx/internal/shared/filesystem"
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

// neverMatcher is a filesystem.IgnoreMatcher that never matches.
type neverMatcher struct{}

func (neverMatcher) Matches(_ string) (bool, error) { return false, nil }

// alwaysMatcher is a filesystem.IgnoreMatcher that always matches.
type alwaysMatcher struct{}

func (alwaysMatcher) Matches(_ string) (bool, error) { return true, nil }

var _ filesystem.IgnoreMatcher = neverMatcher{}
var _ filesystem.IgnoreMatcher = alwaysMatcher{}

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
		initProgress:   disabledInitProgress{},
	}
}

// internalWatchProjectTree implements filesystem.ProjectTree for watch tests.
type internalWatchProjectTree struct{ root string }

func (t *internalWatchProjectTree) CurrentDir() (string, error)          { return t.root, nil }
func (t *internalWatchProjectTree) FindGitRoot(_ string) (string, error) { return t.root, nil }
func (t *internalWatchProjectTree) ReadDir(_ string) ([]filesystem.DirectoryEntry, error) {
	return nil, nil
}
func (t *internalWatchProjectTree) Exists(_ string) (bool, error)      { return false, nil }
func (t *internalWatchProjectTree) WriteFile(_ string, _ []byte) error { return nil }
func (t *internalWatchProjectTree) RemoveAll(_ string) error           { return nil }

// internalWatchMatcherFactory returns a matcher that never ignores.
type internalWatchMatcherFactory struct{}

func (internalWatchMatcherFactory) New(_ string) (filesystem.IgnoreMatcher, error) {
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

func (internalWatchIndexer) BuildIndex(_ []IndexDocument) (*InvertedIndex, error) {
	return NewInvertedIndex(), nil
}

type internalWatchIndexRepo struct{}

func (internalWatchIndexRepo) LoadIndex(_ string) (*InvertedIndex, error) {
	return NewInvertedIndex(), nil
}
func (internalWatchIndexRepo) SaveIndex(_ string, _ *InvertedIndex) error { return nil }

type internalWatchChecksumRepo struct{}

func (internalWatchChecksumRepo) Load(_ string) (map[string]string, bool, error) {
	return nil, false, nil
}
func (internalWatchChecksumRepo) Save(_ string, _ map[string]string) error { return nil }
func (internalWatchChecksumRepo) LoadSnapshot(_ string) (DirectoryChecksumSnapshot, bool, error) {
	return DirectoryChecksumSnapshot{}, false, nil
}
func (internalWatchChecksumRepo) SaveSnapshot(_ string, _ DirectoryChecksumSnapshot) error {
	return nil
}

type internalWatchInspectUI struct{}

func (internalWatchInspectUI) Run(_ *InvertedIndex) error { return nil }
