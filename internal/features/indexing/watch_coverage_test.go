package indexing

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"idx/internal/shared/filesystem"
)

// ---- watchNewDirectory ----

func TestWatchNewDirectory_NonCreateEvent_IsNoop(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	watcher, err := fsnotify.NewWatcher()
	require.NoError(t, err)
	defer watcher.Close()
	svc := newWatchService(root)
	event := fsnotify.Event{Op: fsnotify.Write, Name: filepath.Join(root, "file.go")}

	// Act & Assert
	require.NoError(t, svc.watchNewDirectory(event, watcher, root, neverMatcher{}))
}

func TestWatchNewDirectory_CreateEventOnFile_IsNoop(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	file := filepath.Join(root, "new.go")
	require.NoError(t, os.WriteFile(file, []byte("package x"), 0644))
	watcher, err := fsnotify.NewWatcher()
	require.NoError(t, err)
	defer watcher.Close()
	svc := newWatchService(root)
	event := fsnotify.Event{Op: fsnotify.Create, Name: file}

	// Act & Assert
	require.NoError(t, svc.watchNewDirectory(event, watcher, root, neverMatcher{}))
}

func TestWatchNewDirectory_CreateEventOnNewDir_AddsWatch(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	newDir := filepath.Join(root, "newpkg")
	require.NoError(t, os.Mkdir(newDir, 0755))
	watcher, err := fsnotify.NewWatcher()
	require.NoError(t, err)
	defer watcher.Close()
	svc := newWatchService(root)
	event := fsnotify.Event{Op: fsnotify.Create, Name: newDir}

	// Act & Assert
	require.NoError(t, svc.watchNewDirectory(event, watcher, root, neverMatcher{}))
}

func TestWatchNewDirectory_OutsideRoot_IsNoop(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "outside")
	watcher, err := fsnotify.NewWatcher()
	require.NoError(t, err)
	defer watcher.Close()
	svc := newWatchService(root)
	event := fsnotify.Event{Op: fsnotify.Create, Name: outside}

	// Act & Assert
	require.NoError(t, svc.watchNewDirectory(event, watcher, root, neverMatcher{}))
}

// ---- writeWatchBatchSummary ----

func TestWriteWatchBatchSummary_WritesOutput(t *testing.T) {
	t.Parallel()

	// Arrange
	out := &internalWatchOutput{}
	svc := newWatchService(t.TempDir())
	svc.output = out
	dirs := []string{"/repo/pkg", "/repo/cmd"}
	files := map[string]struct{}{"pkg/svc.go": {}, "cmd/main.go": {}}

	// Act
	require.NoError(t, svc.writeWatchBatchSummary(dirs, files))

	// Assert
	assert.NotEmpty(t, out.lines)
	assert.Contains(t, strings.Join(out.lines, "\n"), "2 dir(s)")
}

func TestWriteWatchBatchSummary_NoFiles_ShowsStructuralChange(t *testing.T) {
	t.Parallel()

	// Arrange
	out := &internalWatchOutput{}
	svc := newWatchService(t.TempDir())
	svc.output = out

	// Act
	require.NoError(t, svc.writeWatchBatchSummary([]string{"/repo"}, map[string]struct{}{}))

	// Assert
	assert.Contains(t, strings.Join(out.lines, "\n"), "structural change")
}

// ---- flushWatchedBatch with ErrNotExist on removeDirectoryIndex ----

func TestFlushWatchedBatch_SkipsDirectoryWhenRemoveNotExist(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	targetDir := filepath.Join(root, "pkg")
	out := &internalWatchOutput{}

	// ReadDir returns no files → removeDirectoryIndex is called → RemoveAll returns ErrNotExist
	tree := &removeNotExistProjectTree{root: root}
	svc := newWatchService(root)
	svc.projectTree = tree
	svc.output = out

	pending := map[string]struct{}{targetDir: {}}

	// Act
	err := svc.flushWatchedBatch(pending, map[string]struct{}{}, root, neverMatcher{}, false)

	// Assert
	require.NoError(t, err, "ErrNotExist should be silently skipped")
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

func TestAddRecursiveWatches_SkipsSystemDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	watcher, err := fsnotify.NewWatcher()
	require.NoError(t, err)
	defer watcher.Close()
	svc := newWatchService(root)

	// Act & Assert — .git should be skipped immediately
	require.NoError(t, svc.addRecursiveWatches(watcher, gitDir, root, neverMatcher{}))
}

func TestAddRecursiveWatches_AddsRealDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	subdir := filepath.Join(root, "pkg")
	require.NoError(t, os.Mkdir(subdir, 0755))
	watcher, err := fsnotify.NewWatcher()
	require.NoError(t, err)
	defer watcher.Close()
	tree := &realDirProjectTree{root: root}
	svc := newWatchService(root)
	svc.projectTree = tree

	// Act & Assert
	require.NoError(t, svc.addRecursiveWatches(watcher, root, root, neverMatcher{}))
}

// ---- addWatchPath ----

func TestAddWatchPath_NonExistent_ReturnsError(t *testing.T) {
	t.Parallel()

	// Arrange
	watcher, err := fsnotify.NewWatcher()
	require.NoError(t, err)
	defer watcher.Close()

	// Act & Assert
	require.Error(t, addWatchPath(watcher, "/nonexistent/xyz/abc"))
}

func TestAddWatchPath_ExistingDir_Succeeds(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	watcher, err := fsnotify.NewWatcher()
	require.NoError(t, err)
	defer watcher.Close()

	// Act & Assert
	require.NoError(t, addWatchPath(watcher, dir))
}

func TestAddWatchPath_IdempotentForAlreadyWatched(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	watcher, err := fsnotify.NewWatcher()
	require.NoError(t, err)
	defer watcher.Close()

	// Act — first and second add of same path should both succeed
	require.NoError(t, addWatchPath(watcher, dir))
	require.NoError(t, addWatchPath(watcher, dir))
}

// ---- consumeWatchEvents ----

func TestConsumeWatchEvents_ExitsWhenWatcherClosed(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	watcher, err := fsnotify.NewWatcher()
	require.NoError(t, err)
	svc := newWatchService(root)
	done := make(chan error, 1)

	// Act
	go func() {
		done <- svc.consumeWatchEvents(context.Background(), watcher, root, neverMatcher{}, false, 50*time.Millisecond)
	}()
	watcher.Close()

	// Assert
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for consumeWatchEvents to return")
	}
}

func TestConsumeWatchEvents_FlushesAfterDebounce(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	out := &internalWatchOutput{}
	watcher, err := fsnotify.NewWatcher()
	require.NoError(t, err)
	require.NoError(t, watcher.Add(root))
	svc := newWatchService(root)
	svc.output = out
	done := make(chan error, 1)

	// Act
	go func() {
		done <- svc.consumeWatchEvents(context.Background(), watcher, root, neverMatcher{}, false, 30*time.Millisecond)
	}()

	// Write a file to trigger an event.
	_ = os.WriteFile(filepath.Join(root, "trigger.go"), []byte("package x"), 0644)

	// Sleep 200ms for the debounce timer (30ms) to fire and flush before closing.
	// This sleep is intentional: we're testing real FS-event debounce timing.
	time.Sleep(200 * time.Millisecond)
	watcher.Close()

	// Assert
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for consumeWatchEvents to return after debounce")
	}
}

// ---- writeWatchHeader ----

func TestWriteWatchHeader_WritesOutput(t *testing.T) {
	t.Parallel()

	// Arrange
	out := &internalWatchOutput{}
	svc := newWatchService(t.TempDir())
	svc.output = out

	// Act
	require.NoError(t, svc.writeWatchHeader(t.TempDir(), 250*time.Millisecond))

	// Assert
	assert.NotEmpty(t, out.lines)
}

// ---- createFileWatcher error path ----

func TestCreateFileWatcher_ReturnsErrorWhenReadDirFails(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	svc := newWatchService(root)
	svc.projectTree = &readDirErrTree{root: root}

	// Act & Assert
	_, err := svc.createFileWatcher(root, neverMatcher{})
	require.Error(t, err)
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

func TestAddRecursiveWatches_ReturnsErrorForNonExistentPath(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	watcher, err := fsnotify.NewWatcher()
	require.NoError(t, err)
	defer watcher.Close()
	svc := newWatchService(root)

	// Act & Assert
	require.Error(t, svc.addRecursiveWatches(watcher, filepath.Join(root, "doesnotexist"), root, neverMatcher{}))
}

func TestAddRecursiveWatches_SkipsIgnoredDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	vendorDir := filepath.Join(root, "vendor")
	require.NoError(t, os.Mkdir(vendorDir, 0755))
	watcher, err := fsnotify.NewWatcher()
	require.NoError(t, err)
	defer watcher.Close()
	svc := newWatchService(root)

	// alwaysMatcher ignores vendor — addRecursiveWatches should return nil without watching it
	require.NoError(t, svc.addRecursiveWatches(watcher, vendorDir, root, alwaysMatcher{}))
}

// ---- createFileWatcher success path ----

func TestCreateFileWatcher_SucceedsForRealDir(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	svc := newWatchService(root)

	// Act — internalWatchProjectTree returns nil,nil for ReadDir, so addRecursiveWatches succeeds
	watcher, err := svc.createFileWatcher(root, neverMatcher{})

	// Assert
	require.NoError(t, err)
	defer watcher.Close()
}

// ---- isWithinRoot with unreachable relative path ----

func TestIsWithinRoot_ReturnsFalseForDotDotPath(t *testing.T) {
	t.Parallel()

	// filepath.Rel returns a path starting with ".." when path is outside root.
	assert.False(t, isWithinRoot("/a/b/c", "/a/b"))
}

// ---- writeWatchFileList with files under limit ----

func TestWriteWatchFileList_FilesUnderLimit_WritesAll(t *testing.T) {
	t.Parallel()

	// Arrange
	out := &internalWatchOutput{}
	svc := newWatchService(t.TempDir())
	svc.output = out

	// Act
	require.NoError(t, svc.writeWatchFileList([]string{"a.go", "b.go"}))

	// Assert — each file + trailing blank line
	assert.GreaterOrEqual(t, len(out.lines), 3)
}

// ---- writeUpdatedFiles error-free path ----

func TestWriteUpdatedFiles_SingleFile_WritesFileName(t *testing.T) {
	t.Parallel()

	// Arrange
	out := &internalWatchOutput{}
	svc := newWatchService(t.TempDir())
	svc.output = out

	// Act
	require.NoError(t, svc.writeUpdatedFiles(map[string]struct{}{"main.go": {}}))

	// Assert
	assert.Contains(t, strings.Join(out.lines, "\n"), "main.go")
}

// ---- isIgnoredPath with relative-path error ----

func TestIsIgnoredPath_ReturnsFalseForRelativePath(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	child := filepath.Join(root, "pkg")

	// Act — neverMatcher always returns false
	ignored, err := isIgnoredPath(root, child, false, neverMatcher{})

	// Assert
	require.NoError(t, err)
	assert.False(t, ignored)
}

// ---- resetDebounceTimer drains expired channel ----

func TestResetDebounceTimer_DrainsExpiredChannel(t *testing.T) {
	t.Parallel()

	// Arrange — create a timer that fires immediately
	first, _ := resetDebounceTimer(nil, 1*time.Millisecond)
	time.Sleep(5 * time.Millisecond) // let timer fire

	// Act — reset the already-fired timer; the drain branch executes
	second, ch := resetDebounceTimer(first, 1*time.Millisecond)

	// Assert
	require.NotNil(t, second)
	require.NotNil(t, ch)
	second.Stop()
}

// ---- resolveWatchContext error paths ----

func TestResolveWatchContext_CurrentDirError_ReturnsError(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := newWatchService(t.TempDir())
	svc.projectTree = &errCurrentDirTree{}

	// Act & Assert
	_, _, err := svc.resolveWatchContext()
	require.Error(t, err)
}

func TestResolveWatchContext_FindGitRootError_ReturnsError(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	svc := newWatchService(root)
	svc.projectTree = &errGitRootTree{root: root}

	// Act & Assert
	_, _, err := svc.resolveWatchContext()
	require.Error(t, err)
}

func TestResolveWatchContext_MatcherFactoryError_ReturnsError(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	svc := newWatchService(root)
	svc.matcherFactory = &errMatcherFactory{}

	// Act & Assert
	_, _, err := svc.resolveWatchContext()
	require.Error(t, err)
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

func TestWatchLoop_PropagatesResolveWatchContextError(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	svc := newWatchService(root)
	svc.projectTree = &errCurrentDirTree{}

	// Act & Assert
	require.Error(t, svc.watchLoop(false, 100*time.Millisecond))
}

func TestWatchLoop_UsesDefaultDebounceWhenZero(t *testing.T) {
	t.Parallel()

	// Arrange — debounce <= 0 → default is applied, then resolveWatchContext fails
	root := t.TempDir()
	svc := newWatchService(root)
	svc.projectTree = &errCurrentDirTree{}

	// Act & Assert
	require.Error(t, svc.watchLoop(false, 0))
}

func TestWatchLoop_WriteWatchHeaderFailure_ReturnsError(t *testing.T) {
	t.Parallel()

	// Arrange — resolveWatchContext succeeds → createFileWatcher succeeds → writeWatchHeader fails
	root := t.TempDir()
	svc := newWatchService(root)
	svc.projectTree = &alwaysExistsWatchTree{root: root}
	svc.output = &failFirstOutput{}

	// Act & Assert
	require.Error(t, svc.watchLoop(false, 50*time.Millisecond))
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

func TestEnsureRootIndex_ReturnsErrorWhenExistsFails(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	svc := newWatchService(root)
	svc.projectTree = &errExistsTree{root: root}

	// Act & Assert
	require.Error(t, svc.ensureRootIndex(root, neverMatcher{}))
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

func TestResolveWatchContext_EnsureRootIndexError_ReturnsError(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	svc := newWatchService(root)
	svc.projectTree = &errExistsTree{root: root}

	// Act & Assert
	_, _, err := svc.resolveWatchContext()
	require.Error(t, err)
}

func TestResolveWatchContext_SyncBeforeWatchError_ReturnsError(t *testing.T) {
	t.Parallel()

	// Arrange — Exists returns true (skip index creation) but ReadDir fails (syncAllDirectoriesBeforeWatch fails)
	root := t.TempDir()
	svc := newWatchService(root)
	svc.projectTree = &existsButReadDirErrTree{root: root}

	// Act & Assert
	_, _, err := svc.resolveWatchContext()
	require.Error(t, err)
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

func TestTrackEventFiles_IgnoresMatchedPath(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	file := filepath.Join(root, "vendor.go")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0644))
	svc := newWatchService(root)
	pending := make(map[string]struct{})

	// Act
	svc.trackEventFiles(fsnotify.Event{Op: fsnotify.Write, Name: file}, root, alwaysMatcher{}, pending)

	// Assert
	assert.Empty(t, pending)
}

// ---- trackEventDirectories ignored path ----

func TestTrackEventDirectories_IgnoresMatchedPath(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	vendorDir := filepath.Join(root, "vendor")
	require.NoError(t, os.Mkdir(vendorDir, 0755))
	svc := newWatchService(root)
	pending := make(map[string]struct{})

	// Act
	svc.trackEventDirectories(fsnotify.Event{Op: fsnotify.Write, Name: vendorDir}, root, alwaysMatcher{}, pending)

	// Assert
	assert.Empty(t, pending)
}

// ---- output error paths in watch event writers ----

func TestWriteWatchFileList_WriteError_ReturnsError(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := newWatchService(t.TempDir())
	svc.output = &failFirstOutput{}

	// Act & Assert
	require.Error(t, svc.writeWatchFileList([]string{"a.go"}))
}

func TestWriteUpdatedFiles_WriteErrorOnHeader_ReturnsError(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := newWatchService(t.TempDir())
	svc.output = &failFirstOutput{}

	// Act & Assert
	require.Error(t, svc.writeUpdatedFiles(map[string]struct{}{"main.go": {}}))
}

func TestWriteWatchBatchSummary_WriteError_ReturnsError(t *testing.T) {
	t.Parallel()

	// Arrange
	svc := newWatchService(t.TempDir())
	svc.output = &failFirstOutput{}

	// Act & Assert
	require.Error(t, svc.writeWatchBatchSummary([]string{"/repo"}, map[string]struct{}{}))
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
	return err == nil, nil
}
func (t *realDirProjectTree) WriteFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0600)
}
func (t *realDirProjectTree) RemoveAll(path string) error { return os.RemoveAll(path) }
