package indexing

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	"idx/internal/shared/filesystem"
)

// ---- watchNewDirectory ----

func TestWatchNewDirectoryNonCreateEventIsNoop(t *testing.T) {
	root := t.TempDir()
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer watcher.Close()

	svc := newWatchService(root)
	event := fsnotify.Event{Op: fsnotify.Write, Name: filepath.Join(root, "file.go")}
	if err := svc.watchNewDirectory(event, watcher, root, neverMatcher{}); err != nil {
		t.Fatalf("unexpected error for non-Create event: %v", err)
	}
}

func TestWatchNewDirectoryCreateEventOnFileIsNoop(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "new.go")
	if err := os.WriteFile(file, []byte("package x"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer watcher.Close()

	svc := newWatchService(root)
	event := fsnotify.Event{Op: fsnotify.Create, Name: file}
	if err := svc.watchNewDirectory(event, watcher, root, neverMatcher{}); err != nil {
		t.Fatalf("unexpected error for Create event on file: %v", err)
	}
}

func TestWatchNewDirectoryCreateEventOnNewDirAddsWatch(t *testing.T) {
	root := t.TempDir()
	newDir := filepath.Join(root, "newpkg")
	if err := os.Mkdir(newDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer watcher.Close()

	svc := newWatchService(root)
	event := fsnotify.Event{Op: fsnotify.Create, Name: newDir}
	if err := svc.watchNewDirectory(event, watcher, root, neverMatcher{}); err != nil {
		t.Fatalf("unexpected error for Create event on directory: %v", err)
	}
}

func TestWatchNewDirectoryOutsideRootIsNoop(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "outside")
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer watcher.Close()

	svc := newWatchService(root)
	event := fsnotify.Event{Op: fsnotify.Create, Name: outside}
	if err := svc.watchNewDirectory(event, watcher, root, neverMatcher{}); err != nil {
		t.Fatalf("unexpected error for outside-root Create: %v", err)
	}
}

// ---- writeWatchBatchSummary ----

func TestWriteWatchBatchSummaryWritesOutput(t *testing.T) {
	out := &internalWatchOutput{}
	svc := newWatchService(t.TempDir())
	svc.output = out

	dirs := []string{"/repo/pkg", "/repo/cmd"}
	files := map[string]struct{}{"pkg/svc.go": {}, "cmd/main.go": {}}

	if err := svc.writeWatchBatchSummary(dirs, files); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.lines) == 0 {
		t.Fatal("expected output from writeWatchBatchSummary")
	}
	joined := strings.Join(out.lines, "\n")
	if !strings.Contains(joined, "2 dir(s)") {
		t.Fatalf("expected dir count in summary, got %q", joined)
	}
}

func TestWriteWatchBatchSummaryNoFilesShowsStructuralChange(t *testing.T) {
	out := &internalWatchOutput{}
	svc := newWatchService(t.TempDir())
	svc.output = out

	if err := svc.writeWatchBatchSummary([]string{"/repo"}, map[string]struct{}{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	joined := strings.Join(out.lines, "\n")
	if !strings.Contains(joined, "structural change") {
		t.Fatalf("expected 'structural change' for empty files, got %q", joined)
	}
}

// ---- flushWatchedBatch with ErrNotExist on removeDirectoryIndex ----

func TestFlushWatchedBatchSkipsDirectoryWhenRemoveNotExist(t *testing.T) {
	root := t.TempDir()
	targetDir := filepath.Join(root, "pkg")
	out := &internalWatchOutput{}

	// ReadDir returns no files → removeDirectoryIndex is called → RemoveAll returns ErrNotExist
	tree := &removeNotExistProjectTree{root: root}
	svc := newWatchService(root)
	svc.projectTree = tree
	svc.output = out

	pending := map[string]struct{}{targetDir: {}}
	err := svc.flushWatchedBatch(pending, map[string]struct{}{}, root, neverMatcher{}, false)
	if err != nil {
		t.Fatalf("expected ErrNotExist to be silently skipped, got %v", err)
	}
}

// removeNotExistProjectTree returns empty ReadDir (no files) and os.ErrNotExist from RemoveAll.
type removeNotExistProjectTree struct{ root string }

func (t *removeNotExistProjectTree) CurrentDir() (string, error)          { return t.root, nil }
func (t *removeNotExistProjectTree) FindGitRoot(_ string) (string, error) { return t.root, nil }
func (t *removeNotExistProjectTree) ReadDir(_ string) ([]filesystem.DirectoryEntry, error) {
	return []filesystem.DirectoryEntry{}, nil
}
func (t *removeNotExistProjectTree) Exists(_ string) (bool, error)      { return false, nil }
func (t *removeNotExistProjectTree) WriteFile(_ string, _ []byte) error { return nil }
func (t *removeNotExistProjectTree) RemoveAll(_ string) error           { return os.ErrNotExist }

// ---- addRecursiveWatches ----

func TestAddRecursiveWatchesSkipsSystemDirectory(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer watcher.Close()

	svc := newWatchService(root)
	// .git should be skipped immediately — no error
	if err := svc.addRecursiveWatches(watcher, gitDir, root, neverMatcher{}); err != nil {
		t.Fatalf("unexpected error for system directory: %v", err)
	}
}

func TestAddRecursiveWatchesAddsRealDirectory(t *testing.T) {
	root := t.TempDir()
	subdir := filepath.Join(root, "pkg")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer watcher.Close()

	tree := &realDirProjectTree{root: root}
	svc := newWatchService(root)
	svc.projectTree = tree

	if err := svc.addRecursiveWatches(watcher, root, root, neverMatcher{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---- addWatchPath ----

func TestAddWatchPathNonExistentReturnsError(t *testing.T) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer watcher.Close()

	err = addWatchPath(watcher, "/nonexistent/xyz/abc")
	if err == nil {
		t.Fatal("expected error for non-existent watch path")
	}
}

func TestAddWatchPathExistingDirSucceeds(t *testing.T) {
	dir := t.TempDir()
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer watcher.Close()

	if err := addWatchPath(watcher, dir); err != nil {
		t.Fatalf("unexpected error for existing dir: %v", err)
	}
}

func TestAddWatchPathIdempotentForAlreadyWatched(t *testing.T) {
	dir := t.TempDir()
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer watcher.Close()

	// First add
	if err := addWatchPath(watcher, dir); err != nil {
		t.Fatalf("unexpected error on first add: %v", err)
	}
	// Second add of same path — should be idempotent
	if err := addWatchPath(watcher, dir); err != nil {
		t.Fatalf("unexpected error on duplicate add: %v", err)
	}
}

// ---- consumeWatchEvents ----

func TestConsumeWatchEventsExitsWhenWatcherClosed(t *testing.T) {
	root := t.TempDir()
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}

	svc := newWatchService(root)
	done := make(chan error, 1)
	go func() {
		done <- svc.consumeWatchEvents(watcher, root, neverMatcher{}, false, 50*time.Millisecond)
	}()

	watcher.Close()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil when watcher is closed, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for consumeWatchEvents to return")
	}
}

func TestConsumeWatchEventsFlushesAfterDebounce(t *testing.T) {
	root := t.TempDir()
	out := &internalWatchOutput{}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	if err := watcher.Add(root); err != nil {
		t.Fatalf("failed to add watch: %v", err)
	}

	svc := newWatchService(root)
	svc.output = out
	done := make(chan error, 1)
	go func() {
		done <- svc.consumeWatchEvents(watcher, root, neverMatcher{}, false, 30*time.Millisecond)
	}()

	// Write a file to trigger an event.
	testFile := filepath.Join(root, "trigger.go")
	_ = os.WriteFile(testFile, []byte("package x"), 0644)

	// Wait enough for the debounce timer to fire and flush, then close the watcher.
	time.Sleep(200 * time.Millisecond)
	watcher.Close()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for consumeWatchEvents to return after debounce")
	}
}

// ---- writeWatchHeader ----

func TestWriteWatchHeaderWritesOutput(t *testing.T) {
	root := t.TempDir()
	out := &internalWatchOutput{}
	svc := newWatchService(root)
	svc.output = out

	if err := svc.writeWatchHeader(root, 250*time.Millisecond); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out.lines) == 0 {
		t.Fatal("expected output from writeWatchHeader")
	}
}

// ---- createFileWatcher error path ----

func TestCreateFileWatcherReturnsErrorWhenReadDirFails(t *testing.T) {
	root := t.TempDir()
	svc := newWatchService(root)
	svc.projectTree = &readDirErrTree{root: root}

	_, err := svc.createFileWatcher(root, neverMatcher{})
	if err == nil {
		t.Fatal("expected error when projectTree.ReadDir fails in addRecursiveWatches")
	}
}

// readDirErrTree fails ReadDir for any path.
type readDirErrTree struct{ root string }

func (t *readDirErrTree) CurrentDir() (string, error)          { return t.root, nil }
func (t *readDirErrTree) FindGitRoot(_ string) (string, error) { return t.root, nil }
func (t *readDirErrTree) ReadDir(_ string) ([]filesystem.DirectoryEntry, error) {
	return nil, errors.New("permission denied")
}
func (t *readDirErrTree) Exists(_ string) (bool, error)      { return false, nil }
func (t *readDirErrTree) WriteFile(_ string, _ []byte) error { return nil }
func (t *readDirErrTree) RemoveAll(_ string) error           { return nil }

// ---- addRecursiveWatches error path ----

func TestAddRecursiveWatchesReturnsErrorForNonExistentPath(t *testing.T) {
	root := t.TempDir()
	nonExistent := filepath.Join(root, "doesnotexist")

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer watcher.Close()

	svc := newWatchService(root)
	err = svc.addRecursiveWatches(watcher, nonExistent, root, neverMatcher{})
	if err == nil {
		t.Fatal("expected error for non-existent directory")
	}
}

func TestAddRecursiveWatchesSkipsIgnoredDirectory(t *testing.T) {
	root := t.TempDir()
	vendorDir := filepath.Join(root, "vendor")
	if err := os.Mkdir(vendorDir, 0755); err != nil {
		t.Fatalf("failed to create vendor: %v", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer watcher.Close()

	svc := newWatchService(root)
	// alwaysMatcher ignores vendor — addRecursiveWatches should return nil without watching it.
	if err := svc.addRecursiveWatches(watcher, vendorDir, root, alwaysMatcher{}); err != nil {
		t.Fatalf("unexpected error for ignored directory: %v", err)
	}
}

// ---- createFileWatcher success path ----

func TestCreateFileWatcherSucceedsForRealDir(t *testing.T) {
	root := t.TempDir()
	svc := newWatchService(root)
	// internalWatchProjectTree returns nil,nil for ReadDir, so addRecursiveWatches succeeds.
	watcher, err := svc.createFileWatcher(root, neverMatcher{})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	defer watcher.Close()
}

// ---- isWithinRoot with unreachable relative path ----

func TestIsWithinRootReturnsFalseForDotDotPath(t *testing.T) {
	// filepath.Rel returns a path starting with ".." when path is outside root.
	if isWithinRoot("/a/b/c", "/a/b") {
		t.Fatal("expected false for parent path")
	}
}

// ---- writeWatchFileList with files under limit ----

func TestWriteWatchFileListWithFilesUnderLimit(t *testing.T) {
	out := &internalWatchOutput{}
	svc := newWatchService(t.TempDir())
	svc.output = out

	files := []string{"a.go", "b.go"}
	if err := svc.writeWatchFileList(files); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Each file + trailing blank line.
	if len(out.lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d: %v", len(out.lines), out.lines)
	}
}

// ---- writeUpdatedFiles error-free path ----

func TestWriteUpdatedFilesWithSingleFile(t *testing.T) {
	out := &internalWatchOutput{}
	svc := newWatchService(t.TempDir())
	svc.output = out

	if err := svc.writeUpdatedFiles(map[string]struct{}{"main.go": {}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	joined := strings.Join(out.lines, "\n")
	if !strings.Contains(joined, "main.go") {
		t.Fatalf("expected file name in output, got %q", joined)
	}
}

// ---- isIgnoredPath with relative-path error (unreachable in practice) ----

func TestIsIgnoredPathReturnsFalseForRelativePath(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "pkg")
	// neverMatcher always returns false.
	ignored, err := isIgnoredPath(root, child, false, neverMatcher{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ignored {
		t.Fatal("expected false from neverMatcher for child path")
	}
}

// ---- resetDebounceTimer drains expired channel ----

func TestResetDebounceTimerDrainsExpiredChannel(t *testing.T) {
	debounce := 1 * time.Millisecond
	// Create a timer that fires immediately.
	first, _ := resetDebounceTimer(nil, debounce)
	time.Sleep(5 * time.Millisecond) // let timer fire
	// Reset the already-fired timer; the drain branch executes.
	second, ch := resetDebounceTimer(first, debounce)
	if second == nil || ch == nil {
		t.Fatal("expected non-nil timer and channel after draining")
	}
	second.Stop()
}

// ---- resolveWatchContext error paths ----

func TestResolveWatchContextCurrentDirError(t *testing.T) {
	tree := &errCurrentDirTree{}
	svc := newWatchService(t.TempDir())
	svc.projectTree = tree

	_, _, err := svc.resolveWatchContext()
	if err == nil {
		t.Fatal("expected error when CurrentDir fails")
	}
}

func TestResolveWatchContextFindGitRootError(t *testing.T) {
	root := t.TempDir()
	tree := &errGitRootTree{root: root}
	svc := newWatchService(root)
	svc.projectTree = tree

	_, _, err := svc.resolveWatchContext()
	if err == nil {
		t.Fatal("expected error when FindGitRoot fails")
	}
}

func TestResolveWatchContextMatcherFactoryError(t *testing.T) {
	root := t.TempDir()
	svc := newWatchService(root)
	svc.matcherFactory = &errMatcherFactory{}

	_, _, err := svc.resolveWatchContext()
	if err == nil {
		t.Fatal("expected error when matcherFactory.New fails")
	}
}

type errCurrentDirTree struct{}

func (errCurrentDirTree) CurrentDir() (string, error) { return "", errors.New("no cwd") }
func (errCurrentDirTree) FindGitRoot(_ string) (string, error) {
	return "", errors.New("no git root")
}
func (errCurrentDirTree) ReadDir(_ string) ([]filesystem.DirectoryEntry, error) { return nil, nil }
func (errCurrentDirTree) Exists(_ string) (bool, error)                         { return false, nil }
func (errCurrentDirTree) WriteFile(_ string, _ []byte) error                    { return nil }
func (errCurrentDirTree) RemoveAll(_ string) error                              { return nil }

type errGitRootTree struct{ root string }

func (t *errGitRootTree) CurrentDir() (string, error) { return t.root, nil }
func (t *errGitRootTree) FindGitRoot(_ string) (string, error) {
	return "", errors.New("not a git repo")
}
func (t *errGitRootTree) ReadDir(_ string) ([]filesystem.DirectoryEntry, error) { return nil, nil }
func (t *errGitRootTree) Exists(_ string) (bool, error)                         { return false, nil }
func (t *errGitRootTree) WriteFile(_ string, _ []byte) error                    { return nil }
func (t *errGitRootTree) RemoveAll(_ string) error                              { return nil }

type errMatcherFactory struct{}

func (errMatcherFactory) New(_ string) (filesystem.IgnoreMatcher, error) {
	return nil, errors.New("gitignore parse error")
}

// ---- watchLoop error propagation from resolveWatchContext ----

func TestWatchLoopPropagatesResolveWatchContextError(t *testing.T) {
	root := t.TempDir()
	svc := newWatchService(root)
	svc.projectTree = &errCurrentDirTree{}

	err := svc.watchLoop(false, 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected error from watchLoop when resolveWatchContext fails")
	}
}

func TestWatchLoopUsesDefaultDebounceWhenZero(t *testing.T) {
	root := t.TempDir()
	svc := newWatchService(root)
	svc.projectTree = &errCurrentDirTree{}

	// debounce <= 0 → default is applied, then resolveWatchContext fails.
	err := svc.watchLoop(false, 0)
	if err == nil {
		t.Fatal("expected error from watchLoop when resolveWatchContext fails")
	}
}

func TestWatchLoopWriteWatchHeaderFailureReturnsError(t *testing.T) {
	root := t.TempDir()
	svc := newWatchService(root)
	// Use a tree where Exists returns true so ensureRootIndex skips index creation.
	svc.projectTree = &alwaysExistsWatchTree{root: root}
	svc.output = &failFirstOutput{}

	// resolveWatchContext succeeds → createFileWatcher succeeds → writeWatchHeader fails.
	err := svc.watchLoop(false, 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected error when writeWatchHeader fails")
	}
}

// alwaysExistsWatchTree returns true for all Exists calls and empty ReadDir.
type alwaysExistsWatchTree struct{ root string }

func (t *alwaysExistsWatchTree) CurrentDir() (string, error)          { return t.root, nil }
func (t *alwaysExistsWatchTree) FindGitRoot(_ string) (string, error) { return t.root, nil }
func (t *alwaysExistsWatchTree) ReadDir(_ string) ([]filesystem.DirectoryEntry, error) {
	return nil, nil
}
func (t *alwaysExistsWatchTree) Exists(_ string) (bool, error)      { return true, nil }
func (t *alwaysExistsWatchTree) WriteFile(_ string, _ []byte) error { return nil }
func (t *alwaysExistsWatchTree) RemoveAll(_ string) error           { return nil }

// failFirstOutput fails on the first call to WriteLine.
type failFirstOutput struct{ called int }

func (o *failFirstOutput) WriteLine(_ string) error {
	o.called++
	if o.called == 1 {
		return errors.New("write error")
	}
	return nil
}

// ---- ensureRootIndex error paths ----

func TestEnsureRootIndexReturnsErrorWhenExistsFails(t *testing.T) {
	root := t.TempDir()
	svc := newWatchService(root)
	svc.projectTree = &errExistsTree{root: root}

	if err := svc.ensureRootIndex(root, neverMatcher{}); err == nil {
		t.Fatal("expected error when Exists fails")
	}
}

// errExistsTree returns an error from Exists.
type errExistsTree struct{ root string }

func (t *errExistsTree) CurrentDir() (string, error)                           { return t.root, nil }
func (t *errExistsTree) FindGitRoot(_ string) (string, error)                  { return t.root, nil }
func (t *errExistsTree) ReadDir(_ string) ([]filesystem.DirectoryEntry, error) { return nil, nil }
func (t *errExistsTree) Exists(_ string) (bool, error) {
	return false, errors.New("stat failed")
}
func (t *errExistsTree) WriteFile(_ string, _ []byte) error { return nil }
func (t *errExistsTree) RemoveAll(_ string) error           { return nil }

// ---- resolveWatchContext additional error paths ----

func TestResolveWatchContextEnsureRootIndexError(t *testing.T) {
	root := t.TempDir()
	svc := newWatchService(root)
	svc.projectTree = &errExistsTree{root: root}

	_, _, err := svc.resolveWatchContext()
	if err == nil {
		t.Fatal("expected error when ensureRootIndex fails")
	}
}

func TestResolveWatchContextSyncBeforeWatchError(t *testing.T) {
	root := t.TempDir()
	svc := newWatchService(root)
	// Exists returns true (skip index creation) but ReadDir fails (syncAllDirectoriesBeforeWatch fails).
	svc.projectTree = &existsButReadDirErrTree{root: root}

	_, _, err := svc.resolveWatchContext()
	if err == nil {
		t.Fatal("expected error when syncAllDirectoriesBeforeWatch fails")
	}
}

// existsButReadDirErrTree returns true for Exists and error for ReadDir.
type existsButReadDirErrTree struct{ root string }

func (t *existsButReadDirErrTree) CurrentDir() (string, error)          { return t.root, nil }
func (t *existsButReadDirErrTree) FindGitRoot(_ string) (string, error) { return t.root, nil }
func (t *existsButReadDirErrTree) ReadDir(_ string) ([]filesystem.DirectoryEntry, error) {
	return nil, errors.New("permission denied")
}
func (t *existsButReadDirErrTree) Exists(_ string) (bool, error)      { return true, nil }
func (t *existsButReadDirErrTree) WriteFile(_ string, _ []byte) error { return nil }
func (t *existsButReadDirErrTree) RemoveAll(_ string) error           { return nil }

// ---- trackEventFiles ignored path ----

func TestTrackEventFilesIgnoresMatchedPath(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "vendor.go")
	if err := os.WriteFile(file, []byte("x"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	svc := newWatchService(root)
	pending := make(map[string]struct{})
	svc.trackEventFiles(fsnotify.Event{Op: fsnotify.Write, Name: file}, root, alwaysMatcher{}, pending)
	if len(pending) != 0 {
		t.Fatal("expected ignored file to not be tracked")
	}
}

// ---- trackEventDirectories ignored path ----

func TestTrackEventDirectoriesIgnoresMatchedPath(t *testing.T) {
	root := t.TempDir()
	vendorDir := filepath.Join(root, "vendor")
	if err := os.Mkdir(vendorDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	svc := newWatchService(root)
	pending := make(map[string]struct{})
	svc.trackEventDirectories(fsnotify.Event{Op: fsnotify.Write, Name: vendorDir}, root, alwaysMatcher{}, pending)
	if len(pending) != 0 {
		t.Fatal("expected ignored dir to not be tracked")
	}
}

// ---- output error paths in watch event writers ----

func TestWriteWatchFileListWriteErrorReturnsError(t *testing.T) {
	svc := newWatchService(t.TempDir())
	svc.output = &failFirstOutput{}

	if err := svc.writeWatchFileList([]string{"a.go"}); err == nil {
		t.Fatal("expected error when WriteLine fails")
	}
}

func TestWriteUpdatedFilesWriteErrorOnHeaderReturnsError(t *testing.T) {
	svc := newWatchService(t.TempDir())
	svc.output = &failFirstOutput{}

	if err := svc.writeUpdatedFiles(map[string]struct{}{"main.go": {}}); err == nil {
		t.Fatal("expected error when header WriteLine fails")
	}
}

func TestWriteWatchBatchSummaryWriteErrorReturnsError(t *testing.T) {
	svc := newWatchService(t.TempDir())
	svc.output = &failFirstOutput{}

	if err := svc.writeWatchBatchSummary([]string{"/repo"}, map[string]struct{}{}); err == nil {
		t.Fatal("expected error when summary WriteLine fails")
	}
}

// realDirProjectTree reads from the real filesystem.
type realDirProjectTree struct{ root string }

func (t *realDirProjectTree) CurrentDir() (string, error)          { return t.root, nil }
func (t *realDirProjectTree) FindGitRoot(_ string) (string, error) { return t.root, nil }
func (t *realDirProjectTree) ReadDir(dir string) ([]filesystem.DirectoryEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	result := make([]filesystem.DirectoryEntry, 0, len(entries))
	for _, e := range entries {
		info, _ := e.Info()
		var size int64
		if info != nil {
			size = info.Size()
		}
		result = append(result, filesystem.DirectoryEntry{
			Name:  e.Name(),
			Path:  filepath.Join(dir, e.Name()),
			IsDir: e.IsDir(),
			Size:  size,
		})
	}
	return result, nil
}
func (t *realDirProjectTree) Exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	return false, nil
}
func (t *realDirProjectTree) WriteFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0600)
}
func (t *realDirProjectTree) RemoveAll(path string) error { return os.RemoveAll(path) }
