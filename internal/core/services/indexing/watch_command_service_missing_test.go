package indexing

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fsnotify/fsnotify"

	"idx/internal/core/domain"
	"idx/internal/core/ports"
)

// --- writeWatchHeader ---

func TestWriteWatchHeaderWritesProjectName(t *testing.T) {
	root := filepath.Join(t.TempDir(), "myproject")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	out := &internalWatchOutput{}
	svc := newWatchService(root)
	svc.output = out

	if err := svc.writeWatchHeader(root, defaultWatchDebounceInterval); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(out.lines) == 0 || !strings.Contains(out.lines[0], "myproject") {
		t.Fatalf("expected project name in header, got %v", out.lines)
	}
}

// --- addRecursiveWatches ---

func TestAddRecursiveWatchesSkipsIgnoredDirectory(t *testing.T) {
	root := t.TempDir()
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer watcher.Close()

	svc := newWatchService(root)
	// alwaysMatcher ignores everything → should return nil without adding watch
	if err := svc.addRecursiveWatches(watcher, root, root, alwaysMatcher{}); err != nil {
		t.Fatalf("expected no error for ignored directory, got %v", err)
	}
}

func TestAddRecursiveWatchesSkipsSystemDirectory(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	if err := os.Mkdir(gitDir, 0755); err != nil {
		t.Fatalf("failed to create .git dir: %v", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer watcher.Close()

	svc := newWatchService(root)
	if err := svc.addRecursiveWatches(watcher, gitDir, root, neverMatcher{}); err != nil {
		t.Fatalf("expected no error for system directory, got %v", err)
	}
}

// --- addWatchPath ---

func TestAddWatchPathSucceedsForValidDirectory(t *testing.T) {
	root := t.TempDir()
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer watcher.Close()

	if err := addWatchPath(watcher, root); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestAddWatchPathHandlesAlreadyWatchedPath(t *testing.T) {
	root := t.TempDir()
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer watcher.Close()

	// Add once to register it, then try again to hit the "exists" branch.
	_ = watcher.Add(root)
	if err := addWatchPath(watcher, root); err != nil {
		t.Fatalf("expected no error for already-watched path, got %v", err)
	}
}

func TestAddWatchPathReturnsErrorForNonexistentDirectory(t *testing.T) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer watcher.Close()

	err = addWatchPath(watcher, "/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Fatal("expected error for nonexistent directory, got nil")
	}
}

// --- resolveWatchContext error branches ---

func TestResolveWatchContextReturnsErrorWhenCurrentDirFails(t *testing.T) {
	svc := newWatchService(t.TempDir())
	svc.projectTree = &errorCurrentDirTree{}

	_, _, err := svc.resolveWatchContext()
	if err == nil {
		t.Fatal("expected error when CurrentDir fails, got nil")
	}
}

func TestResolveWatchContextReturnsErrorWhenMatcherFactoryFails(t *testing.T) {
	root := t.TempDir()
	svc := newWatchService(root)
	svc.matcherFactory = errorMatcherFactory{}

	_, _, err := svc.resolveWatchContext()
	if err == nil {
		t.Fatal("expected error when matcherFactory fails, got nil")
	}
}

type errorCurrentDirTree struct{ internalWatchProjectTree }

func (errorCurrentDirTree) CurrentDir() (string, error) {
	return "", errors.New("no current dir")
}

type errorMatcherFactory struct{}

func (errorMatcherFactory) New(_ string) (ports.IgnoreMatcher, error) {
	return nil, errors.New("matcher factory failed")
}

// --- flushWatchedBatch happy path → covers writeWatchBatchSummary ---

func TestFlushWatchedBatchWithDirectoriesWritesSummary(t *testing.T) {
	root := t.TempDir()
	out := &internalWatchOutput{}
	svc := newWatchService(root)
	svc.output = out

	pending := map[string]struct{}{root: {}}
	files := map[string]struct{}{"main.go": {}}

	if err := svc.flushWatchedBatch(pending, files, root, neverMatcher{}, false); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	joined := strings.Join(out.lines, "\n")
	if !strings.Contains(joined, "dir(s)") {
		t.Fatalf("expected summary with dir count, got %v", out.lines)
	}
}

func TestFlushWatchedBatchSkipsErrNotExistDirectory(t *testing.T) {
	root := t.TempDir()
	out := &internalWatchOutput{}
	svc := newWatchService(root)
	svc.output = out
	svc.indexRepo = &errNotExistIndexRepo{}

	nonexistent := filepath.Join(root, "missing")
	pending := map[string]struct{}{nonexistent: {}}

	// ErrNotExist from indexRepo should be skipped; no summary written for empty result.
	_ = svc.flushWatchedBatch(pending, map[string]struct{}{}, root, neverMatcher{}, false)
}

type errNotExistIndexRepo struct{ internalWatchIndexRepo }

func (errNotExistIndexRepo) LoadIndex(_ string) (*domain.InvertedIndex, error) {
	return nil, os.ErrNotExist
}

// --- watchNewDirectory ---

func TestWatchNewDirectoryIgnoresNonCreateEvent(t *testing.T) {
	root := t.TempDir()
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer watcher.Close()

	svc := newWatchService(root)
	err = svc.watchNewDirectory(fsnotify.Event{Op: fsnotify.Write, Name: root}, watcher, root, neverMatcher{})
	if err != nil {
		t.Fatalf("expected no error for non-create event, got %v", err)
	}
}

func TestWatchNewDirectoryIgnoresNonexistentPath(t *testing.T) {
	root := t.TempDir()
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer watcher.Close()

	svc := newWatchService(root)
	err = svc.watchNewDirectory(
		fsnotify.Event{Op: fsnotify.Create, Name: filepath.Join(root, "missing")},
		watcher, root, neverMatcher{},
	)
	if err != nil {
		t.Fatalf("expected no error for nonexistent path, got %v", err)
	}
}

func TestWatchNewDirectoryIgnoresFile(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "main.go")
	if err := os.WriteFile(file, []byte("package main"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer watcher.Close()

	svc := newWatchService(root)
	err = svc.watchNewDirectory(
		fsnotify.Event{Op: fsnotify.Create, Name: file},
		watcher, root, neverMatcher{},
	)
	if err != nil {
		t.Fatalf("expected no error for file (not dir) event, got %v", err)
	}
}

func TestWatchNewDirectoryAddsWatchForNewDirectory(t *testing.T) {
	root := t.TempDir()
	newDir := filepath.Join(root, "newpkg")
	if err := os.Mkdir(newDir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer watcher.Close()

	svc := newWatchService(root)
	err = svc.watchNewDirectory(
		fsnotify.Event{Op: fsnotify.Create, Name: newDir},
		watcher, root, neverMatcher{},
	)
	if err != nil {
		t.Fatalf("expected no error when adding watch for new directory, got %v", err)
	}
}
