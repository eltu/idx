package indexing

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"idx/internal/shared/filesystem"
)

func TestShouldTrackFileEvent_TrackedOperations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		op      fsnotify.Op
		tracked bool
	}{
		{"Create", fsnotify.Create, true},
		{"Write", fsnotify.Write, true},
		{"Rename", fsnotify.Rename, true},
		{"Remove", fsnotify.Remove, true},
		{"Chmod", fsnotify.Chmod, false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.tracked, shouldTrackFileEvent(tc.op))
		})
	}
}

func TestHasSystemPathSegment_DetectsGitAndIdx(t *testing.T) {
	t.Parallel()

	assert.True(t, hasSystemPathSegment("/repo/.git/refs/HEAD"), "expected .git to be detected")
	assert.True(t, hasSystemPathSegment("/repo/.idx/index.gob"), "expected .idx to be detected")
	assert.False(t, hasSystemPathSegment("/repo/internal/core/service.go"), "expected normal path not to be detected")
}

func TestIsWithinRoot_PathRelationships(t *testing.T) {
	t.Parallel()

	assert.True(t, isWithinRoot("/repo", "/repo/internal/core"), "child should be within root")
	assert.True(t, isWithinRoot("/repo", "/repo"), "root itself should be within root")
	assert.False(t, isWithinRoot("/repo", "/other"), "sibling should not be within root")
	assert.False(t, isWithinRoot("/repo/internal", "/repo"), "parent should not be within child root")
}

func TestShouldSkipSystemDirectory_GitAndIdxSkipped(t *testing.T) {
	t.Parallel()

	assert.True(t, shouldSkipSystemDirectory("/repo/.git"), "expected .git to be skipped")
	assert.True(t, shouldSkipSystemDirectory("/repo/.idx"), "expected .idx to be skipped")
	assert.False(t, shouldSkipSystemDirectory("/repo/internal"), "expected normal directory not to be skipped")
}

func TestSortedDirectoryBatch_ReturnsSorted(t *testing.T) {
	t.Parallel()

	// Arrange
	pending := map[string]struct{}{"/repo/z": {}, "/repo/a": {}, "/repo/m": {}}

	// Act
	result := sortedDirectoryBatch(pending)

	// Assert
	require.Len(t, result, 3)
	assert.Equal(t, "/repo/a", result[0])
	assert.Equal(t, "/repo/m", result[1])
	assert.Equal(t, "/repo/z", result[2])
}

func TestSortedDirectoryBatch_EmptyInput_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	assert.Empty(t, sortedDirectoryBatch(map[string]struct{}{}))
}

func TestSortedFileBatch_ReturnsSorted(t *testing.T) {
	t.Parallel()

	// Arrange
	pending := map[string]struct{}{
		"cmd/main.go":      {},
		"internal/core.go": {},
		"a/b.go":           {},
	}

	// Act
	result := sortedFileBatch(pending)

	// Assert
	require.Len(t, result, 3)
	assert.Equal(t, "a/b.go", result[0])
}

func TestSortedFileBatch_EmptyInput_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	assert.Empty(t, sortedFileBatch(map[string]struct{}{}))
}

func TestResetDebounceTimer_CreatesNewTimerWhenNil(t *testing.T) {
	t.Parallel()

	// Act
	timer, ch := resetDebounceTimer(nil, 50*time.Millisecond)

	// Assert
	require.NotNil(t, timer)
	require.NotNil(t, ch)
	timer.Stop()
}

func TestResetDebounceTimer_ResetsExistingTimer(t *testing.T) {
	t.Parallel()

	// Arrange
	firstTimer, _ := resetDebounceTimer(nil, 50*time.Millisecond)

	// Act
	secondTimer, ch := resetDebounceTimer(firstTimer, 50*time.Millisecond)

	// Assert
	require.NotNil(t, secondTimer)
	require.NotNil(t, ch)
	secondTimer.Stop()
}

func TestIsIgnoredPath_ReturnsFalseForRoot(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()

	// Act
	ignored, err := isIgnoredPath(root, root, true, neverMatcher{})

	// Assert
	require.NoError(t, err)
	assert.False(t, ignored)
}

func TestIsIgnoredPath_DelegatesToMatcher(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	child := filepath.Join(root, "vendor")

	// Act
	ignored, err := isIgnoredPath(root, child, true, alwaysMatcher{})

	// Assert
	require.NoError(t, err)
	assert.True(t, ignored)
}

func TestEventDirectory_ForExistingFile(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	file := filepath.Join(root, "file.go")
	require.NoError(t, os.WriteFile(file, []byte("package main"), 0644))

	// Act
	dir, ok := eventDirectory(root, file)

	// Assert
	require.True(t, ok)
	assert.Equal(t, root, dir)
}

func TestEventDirectory_ForExistingDir(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	child := filepath.Join(root, "subdir")
	require.NoError(t, os.Mkdir(child, 0755))

	// Act
	dir, ok := eventDirectory(root, child)

	// Assert
	require.True(t, ok)
	assert.Equal(t, child, dir)
}

func TestEventDirectory_OutsideRoot_ReturnsFalse(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "outside")

	// Act
	_, ok := eventDirectory(root, outside)

	// Assert
	assert.False(t, ok)
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
