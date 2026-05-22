package indexing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fsnotify/fsnotify"
)

func TestWatchFileLabelEmptyFilesReturnsStructuralChange(t *testing.T) {
	label := watchFileLabel(nil)
	if !strings.Contains(label, "structural change") {
		t.Fatalf("expected 'structural change', got %q", label)
	}
}

func TestWatchFileLabelWithFilesReturnsCount(t *testing.T) {
	label := watchFileLabel([]string{"a.go", "b.go"})
	if !strings.Contains(label, "2 file(s)") {
		t.Fatalf("expected '2 file(s)', got %q", label)
	}
}

func TestTruncatedFileListUnderLimitReturnsAll(t *testing.T) {
	files := []string{"a.go", "b.go", "c.go"}
	listed, truncated := truncatedFileList(files)
	if len(listed) != 3 || truncated != 0 {
		t.Fatalf("expected 3 listed and 0 truncated, got %d and %d", len(listed), truncated)
	}
}

func TestTruncatedFileListOverLimitTruncates(t *testing.T) {
	files := make([]string, watchMaxFilesListed+3)
	for i := range files {
		files[i] = "file.go"
	}
	listed, truncated := truncatedFileList(files)
	if len(listed) != watchMaxFilesListed {
		t.Fatalf("expected %d listed, got %d", watchMaxFilesListed, len(listed))
	}
	if truncated != 3 {
		t.Fatalf("expected 3 truncated, got %d", truncated)
	}
}

func TestWatchEntryPrefixLastEntryNoTruncation(t *testing.T) {
	prefix := watchEntryPrefix(2, 2, 0)
	if !strings.Contains(prefix, "└─") {
		t.Fatalf("expected └─ for last entry with no truncation, got %q", prefix)
	}
}

func TestWatchEntryPrefixMiddleEntry(t *testing.T) {
	prefix := watchEntryPrefix(0, 2, 0)
	if !strings.Contains(prefix, "├─") {
		t.Fatalf("expected ├─ for non-last entry, got %q", prefix)
	}
}

func TestWatchEntryPrefixLastEntryWithTruncation(t *testing.T) {
	prefix := watchEntryPrefix(2, 2, 1)
	if !strings.Contains(prefix, "├─") {
		t.Fatalf("expected ├─ for last listed entry when truncation follows, got %q", prefix)
	}
}

func TestWriteUpdatedFilesEmptyWritesNone(t *testing.T) {
	out := &internalWatchOutput{}
	svc := newWatchService(t.TempDir())
	svc.output = out

	if err := svc.writeUpdatedFiles(map[string]struct{}{}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(out.lines) == 0 || !strings.Contains(out.lines[0], "none") {
		t.Fatalf("expected 'none' line, got %v", out.lines)
	}
}

func TestWriteUpdatedFilesWithFilesWritesList(t *testing.T) {
	out := &internalWatchOutput{}
	svc := newWatchService(t.TempDir())
	svc.output = out

	pending := map[string]struct{}{"main.go": {}, "util.go": {}}
	if err := svc.writeUpdatedFiles(pending); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	joined := strings.Join(out.lines, "\n")
	if !strings.Contains(joined, "updated files") {
		t.Fatalf("expected 'updated files' header, got %v", out.lines)
	}
}

func TestWriteWatchFileListEmptyWritesBlankLine(t *testing.T) {
	out := &internalWatchOutput{}
	svc := newWatchService(t.TempDir())
	svc.output = out

	if err := svc.writeWatchFileList(nil); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(out.lines) != 1 || out.lines[0] != "" {
		t.Fatalf("expected single blank line, got %v", out.lines)
	}
}

func TestWriteWatchFileListWithFilesTruncates(t *testing.T) {
	out := &internalWatchOutput{}
	svc := newWatchService(t.TempDir())
	svc.output = out

	files := make([]string, watchMaxFilesListed+2)
	for i := range files {
		files[i] = "file.go"
	}

	if err := svc.writeWatchFileList(files); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	joined := strings.Join(out.lines, "\n")
	if !strings.Contains(joined, "and 2 more") {
		t.Fatalf("expected truncation message, got %v", out.lines)
	}
}

func TestTrackEventFilesIgnoresChmod(t *testing.T) {
	root := t.TempDir()
	svc := newWatchService(root)
	pending := map[string]struct{}{}

	svc.trackEventFiles(fsnotify.Event{Op: fsnotify.Chmod, Name: filepath.Join(root, "main.go")}, root, neverMatcher{}, pending)
	if len(pending) != 0 {
		t.Fatal("expected Chmod to not add to pending files")
	}
}

func TestTrackEventFilesIgnoresOutsideRoot(t *testing.T) {
	root := t.TempDir()
	svc := newWatchService(root)
	pending := map[string]struct{}{}

	svc.trackEventFiles(fsnotify.Event{Op: fsnotify.Write, Name: "/outside/file.go"}, root, neverMatcher{}, pending)
	if len(pending) != 0 {
		t.Fatal("expected outside-root path to not be tracked")
	}
}

func TestTrackEventFilesIgnoresDirectory(t *testing.T) {
	root := t.TempDir()
	subdir := filepath.Join(root, "sub")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}
	svc := newWatchService(root)
	pending := map[string]struct{}{}

	svc.trackEventFiles(fsnotify.Event{Op: fsnotify.Write, Name: subdir}, root, neverMatcher{}, pending)
	if len(pending) != 0 {
		t.Fatal("expected directory event to not be tracked as file")
	}
}

func TestTrackEventFilesIgnoresSystemPaths(t *testing.T) {
	root := t.TempDir()
	svc := newWatchService(root)
	pending := map[string]struct{}{}

	svc.trackEventFiles(fsnotify.Event{Op: fsnotify.Write, Name: filepath.Join(root, ".git", "COMMIT_EDITMSG")}, root, neverMatcher{}, pending)
	if len(pending) != 0 {
		t.Fatal("expected .git path to not be tracked")
	}
}

func TestTrackEventDirectoriesIgnoresChmod(t *testing.T) {
	root := t.TempDir()
	svc := newWatchService(root)
	pending := map[string]struct{}{}

	svc.trackEventDirectories(fsnotify.Event{Op: fsnotify.Chmod, Name: root}, root, neverMatcher{}, pending)
	if len(pending) != 0 {
		t.Fatal("expected Chmod to not add to pending directories")
	}
}

func TestTrackEventDirectoriesIgnoresSystemDirectory(t *testing.T) {
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	if err := os.Mkdir(gitDir, 0755); err != nil {
		t.Fatalf("failed to create .git dir: %v", err)
	}
	svc := newWatchService(root)
	pending := map[string]struct{}{}

	svc.trackEventDirectories(fsnotify.Event{Op: fsnotify.Write, Name: gitDir}, root, neverMatcher{}, pending)
	if len(pending) != 0 {
		t.Fatal("expected .git directory to not be tracked")
	}
}

func TestTrackEventDirectoriesIgnoresOutsideRoot(t *testing.T) {
	root := t.TempDir()
	svc := newWatchService(root)
	pending := map[string]struct{}{}

	svc.trackEventDirectories(fsnotify.Event{Op: fsnotify.Write, Name: "/outside/dir"}, root, neverMatcher{}, pending)
	if len(pending) != 0 {
		t.Fatal("expected outside-root path to not be tracked")
	}
}
