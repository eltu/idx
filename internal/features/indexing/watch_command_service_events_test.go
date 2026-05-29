package indexing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const watchTestFileName = "file.go"

func TestWatchFileLabel_EmptyFiles_ReturnsStructuralChange(t *testing.T) {
	t.Parallel()

	assert.Contains(t, watchFileLabel(nil), "structural change")
}

func TestWatchFileLabel_WithFiles_ReturnsCount(t *testing.T) {
	t.Parallel()

	assert.Contains(t, watchFileLabel([]string{"a.go", "b.go"}), "2 file(s)")
}

func TestTruncatedFileList_UnderLimit_ReturnsAll(t *testing.T) {
	t.Parallel()

	// Arrange
	files := []string{"a.go", "b.go", "c.go"}

	// Act
	listed, truncated := truncatedFileList(files)

	// Assert
	assert.Len(t, listed, 3)
	assert.Equal(t, 0, truncated)
}

func TestTruncatedFileList_OverLimit_Truncates(t *testing.T) {
	t.Parallel()

	// Arrange
	files := make([]string, watchMaxFilesListed+3)
	for i := range files {
		files[i] = watchTestFileName
	}

	// Act
	listed, truncated := truncatedFileList(files)

	// Assert
	assert.Len(t, listed, watchMaxFilesListed)
	assert.Equal(t, 3, truncated)
}

func TestWatchEntryPrefix_LastEntryNoTruncation_ReturnsCornerBranch(t *testing.T) {
	t.Parallel()

	assert.Contains(t, watchEntryPrefix(2, 2, 0), "└─")
}

func TestWatchEntryPrefix_MiddleEntry_ReturnsTBranch(t *testing.T) {
	t.Parallel()

	assert.Contains(t, watchEntryPrefix(0, 2, 0), "├─")
}

func TestWatchEntryPrefix_LastEntryWithTruncation_ReturnsTBranch(t *testing.T) {
	t.Parallel()

	assert.Contains(t, watchEntryPrefix(2, 2, 1), "├─")
}

func TestWriteUpdatedFiles_Empty_WritesNoneLine(t *testing.T) {
	t.Parallel()

	// Arrange
	out := &internalWatchOutput{}
	svc := newWatchService(t.TempDir())
	svc.output = out

	// Act
	require.NoError(t, svc.writeUpdatedFiles(map[string]struct{}{}))

	// Assert
	require.NotEmpty(t, out.lines)
	assert.Contains(t, out.lines[0], "none")
}

func TestWriteUpdatedFiles_WithFiles_WritesHeader(t *testing.T) {
	t.Parallel()

	// Arrange
	out := &internalWatchOutput{}
	svc := newWatchService(t.TempDir())
	svc.output = out

	// Act
	require.NoError(t, svc.writeUpdatedFiles(map[string]struct{}{"main.go": {}, "util.go": {}}))

	// Assert
	joined := strings.Join(out.lines, "\n")
	assert.Contains(t, joined, "updated files")
}

func TestWriteWatchFileList_Empty_WritesBlankLine(t *testing.T) {
	t.Parallel()

	// Arrange
	out := &internalWatchOutput{}
	svc := newWatchService(t.TempDir())
	svc.output = out

	// Act
	require.NoError(t, svc.writeWatchFileList(nil))

	// Assert
	require.Len(t, out.lines, 1)
	assert.Equal(t, "", out.lines[0])
}

func TestWriteWatchFileList_OverLimit_WritesTruncationMessage(t *testing.T) {
	t.Parallel()

	// Arrange
	out := &internalWatchOutput{}
	svc := newWatchService(t.TempDir())
	svc.output = out
	files := make([]string, watchMaxFilesListed+2)
	for i := range files {
		files[i] = watchTestFileName
	}

	// Act
	require.NoError(t, svc.writeWatchFileList(files))

	// Assert
	assert.Contains(t, strings.Join(out.lines, "\n"), "and 2 more")
}

func TestTrackEventFiles_IgnoresChmod(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	svc := newWatchService(root)
	pending := map[string]struct{}{}

	// Act
	svc.trackEventFiles(fsnotify.Event{Op: fsnotify.Chmod, Name: filepath.Join(root, "main.go")}, root, neverMatcher{}, pending)

	// Assert
	assert.Empty(t, pending)
}

func TestTrackEventFiles_IgnoresOutsideRoot(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	svc := newWatchService(root)
	pending := map[string]struct{}{}

	// Act
	svc.trackEventFiles(fsnotify.Event{Op: fsnotify.Write, Name: "/outside/file.go"}, root, neverMatcher{}, pending)

	// Assert
	assert.Empty(t, pending)
}

func TestTrackEventFiles_IgnoresDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	subdir := filepath.Join(root, "sub")
	require.NoError(t, os.Mkdir(subdir, 0755))
	svc := newWatchService(root)
	pending := map[string]struct{}{}

	// Act
	svc.trackEventFiles(fsnotify.Event{Op: fsnotify.Write, Name: subdir}, root, neverMatcher{}, pending)

	// Assert
	assert.Empty(t, pending)
}

func TestTrackEventFiles_IgnoresSystemPaths(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	svc := newWatchService(root)
	pending := map[string]struct{}{}

	// Act
	svc.trackEventFiles(fsnotify.Event{Op: fsnotify.Write, Name: filepath.Join(root, ".git", "COMMIT_EDITMSG")}, root, neverMatcher{}, pending)

	// Assert
	assert.Empty(t, pending)
}

func TestTrackEventDirectories_IgnoresChmod(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	svc := newWatchService(root)
	pending := map[string]struct{}{}

	// Act
	svc.trackEventDirectories(fsnotify.Event{Op: fsnotify.Chmod, Name: root}, root, neverMatcher{}, pending)

	// Assert
	assert.Empty(t, pending)
}

func TestTrackEventDirectories_IgnoresSystemDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	gitDir := filepath.Join(root, ".git")
	require.NoError(t, os.Mkdir(gitDir, 0755))
	svc := newWatchService(root)
	pending := map[string]struct{}{}

	// Act
	svc.trackEventDirectories(fsnotify.Event{Op: fsnotify.Write, Name: gitDir}, root, neverMatcher{}, pending)

	// Assert
	assert.Empty(t, pending)
}

func TestTrackEventDirectories_IgnoresOutsideRoot(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	svc := newWatchService(root)
	pending := map[string]struct{}{}

	// Act
	svc.trackEventDirectories(fsnotify.Event{Op: fsnotify.Write, Name: "/outside/dir"}, root, neverMatcher{}, pending)

	// Assert
	assert.Empty(t, pending)
}
