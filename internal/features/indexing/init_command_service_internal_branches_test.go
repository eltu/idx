package indexing

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"idx/internal/shared/filesystem"
)

func TestInitCommandService_BuildAndSaveIndex_ErrorBranches(t *testing.T) {
	t.Parallel()

	// Arrange
	root := filepath.Join(string(filepath.Separator), "repo")
	entry := filesystem.DirectoryEntry{Name: "a.txt", Path: filepath.Join(root, "a.txt")}
	snapshot := DirectoryChecksumSnapshot{Files: map[string]FileChecksumState{}}

	// Act / Assert: file read error
	svc1 := newValidInternalService(root)
	svc1.fileReader = internalFileReader{err: errors.New("read failure")}
	require.Error(t, svc1.buildAndSaveIndex(root, []filesystem.DirectoryEntry{entry}, snapshot, map[string]struct{}{}))

	// Act / Assert: indexer error
	svc2 := newValidInternalService(root)
	svc2.fileReader = internalFileReader{files: map[string]string{entry.Path: "hello"}}
	svc2.indexer = internalIndexer{err: errors.New("indexer failure")}
	require.Error(t, svc2.buildAndSaveIndex(root, []filesystem.DirectoryEntry{entry}, snapshot, map[string]struct{}{}))

	// Act / Assert: save error
	svc3 := newValidInternalService(root)
	svc3.fileReader = internalFileReader{files: map[string]string{entry.Path: "hello"}}
	svc3.indexRepo = internalIndexRepo{saveErr: errors.New("save failure")}
	require.Error(t, svc3.buildAndSaveIndex(root, []filesystem.DirectoryEntry{entry}, snapshot, map[string]struct{}{}))
}

func TestInitCommandService_ShouldReindexDirectory_ErrorBranches(t *testing.T) {
	t.Parallel()

	// Arrange
	root := filepath.Join(string(filepath.Separator), "repo")

	// Act / Assert: hasDirectoryIndex stat error
	svc1 := newValidInternalService(root)
	svc1.projectTree.(*internalProjectTree).existsErr[indexFilePath(root)] = errors.New("exists failure")
	_, err := svc1.shouldReindexDirectory(root, map[string]string{})
	require.Error(t, err)

	// Act / Assert: checksum load error
	svc2 := newValidInternalService(root)
	svc2.checksumRepo = internalChecksumRepo{loadErr: errors.New("load failure")}
	_, err = svc2.shouldReindexDirectory(root, map[string]string{})
	require.Error(t, err)

	// Act / Assert: no checksums → must reindex
	svc3 := newValidInternalService(root)
	svc3.checksumRepo = internalChecksumRepo{loadData: map[string]string{}, exists: false}
	should, err := svc3.shouldReindexDirectory(root, map[string]string{})
	require.NoError(t, err)
	assert.True(t, should, "expected reindex when checksums are not present")
}

func TestInitCommandService_Helpers_CoverUncoveredBranches(t *testing.T) {
	t.Parallel()

	// Arrange
	root := filepath.Join(string(filepath.Separator), "repo")
	indexedAt := time.Unix(1700000000, 0).UTC()

	// buildChangedLogEntries with nil entries
	assert.Empty(t, buildChangedLogEntries(nil, DirectoryChecksumSnapshot{Files: map[string]FileChecksumState{}}, map[string]struct{}{}, indexedAt))

	// buildChangedLogEntries skips when snapshot state missing
	entries := []filesystem.DirectoryEntry{{Name: "a.txt", Path: filepath.Join(root, "a.txt")}}
	snapshot := DirectoryChecksumSnapshot{Files: map[string]FileChecksumState{}}
	changed := map[string]struct{}{"a.txt": {}}
	assert.Empty(t, buildChangedLogEntries(entries, snapshot, changed, indexedAt), "expected skip when snapshot state missing")

	// filterEntries matcher error
	_, err := filterEntries([]filesystem.DirectoryEntry{{Name: "a.txt", Path: filepath.Join(root, "a.txt")}}, root, internalMatcher{err: errors.New("matcher failure")})
	require.Error(t, err)

	// sameChecksums: different map sizes
	assert.False(t, sameChecksums(map[string]string{"a": "1"}, map[string]string{"a": "1", "b": "2"}))

	// sameSnapshotChecksums: differing checksum value
	assert.False(t, sameSnapshotChecksums(map[string]FileChecksumState{"a": {Checksum: "1"}}, map[string]FileChecksumState{"a": {Checksum: "2"}}))

	// filterEntries skips symlinks
	matcher := &countingMatcher{}
	filtered, err := filterEntries([]filesystem.DirectoryEntry{
		{Name: "link", Path: filepath.Join(root, "link"), IsSymlink: true},
		{Name: "file.txt", Path: filepath.Join(root, "file.txt")},
	}, root, matcher)
	require.NoError(t, err)
	require.Len(t, filtered, 1)
	assert.Equal(t, "file.txt", filtered[0].Name)
	assert.Equal(t, 1, matcher.calls, "expected matcher to be called only for non-symlink entries")
}

func TestInitCommandService_DirectoryChecksumsAndSameChecksums_Branches(t *testing.T) {
	t.Parallel()

	// Arrange
	root := filepath.Join(string(filepath.Separator), "repo")
	service := newValidInternalService(root)

	firstPath := filepath.Join(root, "a.txt")
	secondPath := filepath.Join(root, "b.txt")
	service.fileReader = internalFileReader{files: map[string]string{firstPath: "a", secondPath: "b"}}

	// Act
	checksums, err := service.directoryChecksums([]filesystem.DirectoryEntry{
		{Name: "a.txt", Path: firstPath},
		{Name: "b.txt", Path: secondPath},
	})

	// Assert
	require.NoError(t, err)
	assert.Len(t, checksums, 2)

	// read error
	service.fileReader = internalFileReader{err: errors.New("read failure")}
	_, err = service.directoryChecksums([]filesystem.DirectoryEntry{{Name: "a.txt", Path: firstPath}})
	require.Error(t, err)

	// sameChecksums variants
	assert.True(t, sameChecksums(map[string]string{"a": "1"}, map[string]string{"a": "1"}))
	assert.False(t, sameChecksums(map[string]string{"a": "1"}, map[string]string{"b": "1"}))
	assert.False(t, sameChecksums(map[string]string{"a": "1"}, map[string]string{"a": "2"}))
}

func TestInitCommandService_LoadAndSaveChecksumSnapshot_Branches(t *testing.T) {
	t.Parallel()

	// Arrange
	root := filepath.Join(string(filepath.Separator), "repo")
	service := newValidInternalService(root)

	// Act / Assert: legacy checksum repo load
	service.checksumRepo = internalChecksumRepo{loadData: map[string]string{"a.txt": "hash"}, exists: true}
	snapshot, exists, err := service.loadChecksumSnapshot(root)
	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "hash", snapshot.Files["a.txt"].Checksum)

	// Act / Assert: legacy save error
	service.checksumRepo = internalChecksumRepo{saveErr: errors.New("save failure")}
	err = service.saveChecksumSnapshot(root, DirectoryChecksumSnapshot{Files: map[string]FileChecksumState{"a.txt": {Checksum: "hash"}}})
	require.Error(t, err)

	// Act / Assert: snapshot repo load
	service.checksumRepo = internalSnapshotChecksumRepo{loadSnapshotResult: DirectoryChecksumSnapshot{Files: map[string]FileChecksumState{"a.txt": {Checksum: "hash"}}}, loadSnapshotExists: true}
	snapshot, exists, err = service.loadChecksumSnapshot(root)
	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "hash", snapshot.Files["a.txt"].Checksum)

	// Act / Assert: snapshot save error
	service.checksumRepo = internalSnapshotChecksumRepo{saveSnapshotErr: errors.New("snapshot save failure")}
	err = service.saveChecksumSnapshot(root, DirectoryChecksumSnapshot{Files: map[string]FileChecksumState{}})
	require.Error(t, err)
}

func TestInitCommandService_ShouldReindexDirectoryAndMetadata_Branches(t *testing.T) {
	t.Parallel()

	// Arrange
	root := filepath.Join(string(filepath.Separator), "repo")
	service := newValidInternalService(root)
	tree := service.projectTree.(*internalProjectTree)

	// Act / Assert: no index → must reindex
	tree.existsMap[indexFilePath(root)] = false
	should, err := service.shouldReindexDirectory(root, map[string]string{})
	require.NoError(t, err)
	assert.True(t, should)

	// Act / Assert: equal checksums → no reindex
	tree.existsMap[indexFilePath(root)] = true
	service.checksumRepo = internalChecksumRepo{loadData: map[string]string{"a": "1"}, exists: true}
	should, err = service.shouldReindexDirectory(root, map[string]string{"a": "1"})
	require.NoError(t, err)
	assert.False(t, should)

	// Act / Assert: different checksums → reindex
	should, err = service.shouldReindexDirectory(root, map[string]string{"a": "2"})
	require.NoError(t, err)
	assert.True(t, should)

	// metadataUnchanged: identical metadata
	assert.True(t, metadataUnchanged(filesystem.DirectoryEntry{Size: 10, ModTimeUnixNano: 20}, FileChecksumState{Size: 10, ModTimeUnixNano: 20}))
}

func TestInitCommandService_RemoveDirectoryIndex_ErrorBranch(t *testing.T) {
	t.Parallel()

	// Arrange
	root := filepath.Join(string(filepath.Separator), "repo")
	service := newValidInternalService(root)
	service.projectTree.(*internalProjectTree).removeErr = errors.New("remove failure")

	// Act
	err := service.removeDirectoryIndex(root)

	// Assert
	require.Error(t, err)
}

func TestCloneInspectDocTermStats_NilInput_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	// Act
	got := cloneInspectDocTermStats(nil)

	// Assert
	require.NotNil(t, got)
	assert.Equal(t, 0, got.TF)
	assert.Empty(t, got.Positions)
}

func TestCloneInspectDocTermStats_CopiesPositions(t *testing.T) {
	t.Parallel()

	// Arrange
	original := &DocTermStats{TF: 3, Positions: []int{1, 4, 7}}

	// Act
	cloned := cloneInspectDocTermStats(original)

	// Assert
	assert.Equal(t, 3, cloned.TF)
	require.Len(t, cloned.Positions, 3)
	assert.Equal(t, 1, cloned.Positions[0])
	assert.Equal(t, 7, cloned.Positions[2])

	// Verify deep copy
	original.Positions[0] = 99
	assert.NotEqual(t, 99, cloned.Positions[0], "expected deep copy of positions slice, but clone was mutated")
}

func TestRunInspectTUIForDirectory_CallsInspectUI(t *testing.T) {
	t.Parallel()

	// Arrange
	root := filepath.Join(string(filepath.Separator), "repo")
	called := false
	service := newValidInternalService(root)
	service.inspectUI = internalInspectUIRunnerFunc(func(_ *InvertedIndex) error {
		called = true
		return nil
	})

	// Act
	err := service.runInspectTUIForDirectory(root)

	// Assert
	require.NoError(t, err)
	assert.True(t, called)
}

func TestRunInspectTUIForDirectory_PropagatesIndexLoadError(t *testing.T) {
	t.Parallel()

	// Arrange
	root := filepath.Join(string(filepath.Separator), "repo")
	service := newValidInternalService(root)
	service.indexRepo = internalIndexRepo{loadErr: errors.New("load failure")}

	// Act
	err := service.runInspectTUIForDirectory(root)

	// Assert
	require.Error(t, err)
}

type internalInspectUIRunnerFunc func(*InvertedIndex) error

func (fn internalInspectUIRunnerFunc) Run(index *InvertedIndex) error { return fn(index) }

func TestMergeInspectTerms_CopiesTermsWithNamespacedDocIDs(t *testing.T) {
	t.Parallel()

	// Arrange
	source := NewInvertedIndex()
	source.AddDocument("service.go", "internal/service.go", 10)
	source.AddTerm("alpha", "service.go", 2, []int{1, 5})

	target := NewInvertedIndex()

	// Act
	mergeInspectTerms(target, "/repo/internal", source)

	// Assert
	termStats, ok := target.Terms["alpha"]
	require.True(t, ok, "expected term 'alpha' to be merged into target")

	expectedDocID := "/repo/internal::service.go"
	assert.Contains(t, termStats.Docs, expectedDocID)
}

func TestMergeInspectTerms_SkipsNilTermStats(t *testing.T) {
	t.Parallel()

	// Arrange
	source := NewInvertedIndex()
	source.Terms["alpha"] = nil

	target := NewInvertedIndex()

	// Act
	mergeInspectTerms(target, "/repo", source)

	// Assert
	assert.NotContains(t, target.Terms, "alpha")
}

func TestRunInspectTUIForDirectories_CallsInspectUI(t *testing.T) {
	t.Parallel()

	// Arrange
	root := filepath.Join(string(filepath.Separator), "repo")
	called := false
	service := newValidInternalService(root)
	service.indexRepo = internalIndexRepo{index: NewInvertedIndex()}
	service.inspectUI = internalInspectUIRunnerFunc(func(_ *InvertedIndex) error {
		called = true
		return nil
	})

	// Act
	err := service.runInspectTUIForDirectories([]string{root})

	// Assert
	require.NoError(t, err)
	assert.True(t, called)
}

func TestRunInspectTUIForDirectories_PropagatesLoadError(t *testing.T) {
	t.Parallel()

	// Arrange
	root := filepath.Join(string(filepath.Separator), "repo")
	service := newValidInternalService(root)
	service.indexRepo = internalIndexRepo{loadErr: errors.New("load failure")}

	// Act
	err := service.runInspectTUIForDirectories([]string{root})

	// Assert
	require.Error(t, err)
}

func TestWatch_ZeroDebounce_ReturnsError(t *testing.T) {
	t.Parallel()

	// Arrange
	root := filepath.Join(string(filepath.Separator), "repo")
	service := newValidInternalService(root)

	// Act
	err := service.Watch(false, 0)

	// Assert
	require.Error(t, err)
}

func TestWatch_DaemonAlreadyMonitoring_ReturnsError(t *testing.T) {
	t.Parallel()

	// Arrange
	root := filepath.Join(string(filepath.Separator), "repo")
	service := newValidInternalService(root)
	service.daemonRepo = internalDaemonRepo{monitoredPaths: map[string]bool{root: true}}

	// Act
	err := service.Watch(false, time.Second)

	// Assert
	require.Error(t, err)
}

func TestWatch_StartedByDaemon_SkipsDaemonBlock(t *testing.T) {
	// Arrange
	root := filepath.Join(string(filepath.Separator), "repo")
	service := newValidInternalService(root)
	service.daemonRepo = internalDaemonRepo{monitoredPaths: map[string]bool{root: true}}
	t.Setenv(daemonChildEnvVar, "1")

	// Act
	err := service.Watch(false, time.Second)

	// Assert: daemon child watch must bypass daemon self-check
	if err != nil {
		assert.NotContains(t, err.Error(), "cannot run watch: daemon is already monitoring this project")
	}
}

func TestWatch_OtherMonitoredProject_DoesNotBlock(t *testing.T) {
	t.Parallel()

	// Arrange
	root := filepath.Join(string(filepath.Separator), "repo")
	service := newValidInternalService(root)
	service.daemonRepo = internalDaemonRepo{monitoredPaths: map[string]bool{filepath.Join(root, "other"): true}}

	// Act
	err := service.Watch(false, time.Second)

	// Assert
	if err != nil {
		assert.NotContains(t, err.Error(), "cannot run watch: daemon is already monitoring this project")
	}
}

func TestWatch_NilDaemonRepo_DoesNotPanic(t *testing.T) {
	t.Parallel()

	// Arrange
	root := filepath.Join(string(filepath.Separator), "repo")
	service := newValidInternalService(root)
	service.daemonRepo = nil

	// Act / Assert: must not panic
	err := service.Watch(false, time.Second)
	_ = err
}

func TestIsMissingFileError_NotExistError_ReturnsTrue(t *testing.T) {
	t.Parallel()
	assert.True(t, isMissingFileError(os.ErrNotExist))
}

func TestIsMissingFileError_FileNotFoundMessage_ReturnsTrue(t *testing.T) {
	t.Parallel()
	assert.True(t, isMissingFileError(errors.New("file not found")))
}

func TestIsMissingFileError_NoSuchFileMessage_ReturnsTrue(t *testing.T) {
	t.Parallel()
	assert.True(t, isMissingFileError(errors.New("no such file or directory")))
}

func TestIsMissingFileError_OtherError_ReturnsFalse(t *testing.T) {
	t.Parallel()
	assert.False(t, isMissingFileError(errors.New("some other error")))
}

func TestInitCommandService_InspectAndWriteInspectIndex_ErrorBranchesFromMovedFile(t *testing.T) {
	t.Parallel()

	// Arrange
	root := filepath.Join(string(filepath.Separator), "repo")
	service := newValidInternalService(root)
	tree := service.projectTree.(*internalProjectTree)

	// Exists error during Inspect
	tree.existsErr[indexFilePath(root)] = errors.New("exists failure")
	require.Error(t, service.Inspect("."))

	// Load error in LoadInspectIndex (single directory path)
	service.indexRepo = internalIndexRepo{loadErr: errors.New("load failure")}
	_, err := service.LoadInspectIndex(".")
	require.Error(t, err)

	// JSON marshal error for NaN in writeIndexJSON
	invalid := NewInvertedIndex()
	invalid.AverageDocLength = math.NaN()
	require.Error(t, service.writeIndexJSON(invalid))
}

// keep strings imported to allow error-message checks in the Test functions above.
var _ = strings.Contains
