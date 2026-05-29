package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"idx/internal/features/indexing"
)

func TestDirectoryChecksumRepository_Load_UsesInMemoryCacheWhenFileUnchanged(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := NewDirectoryChecksumRepository()
	dir := t.TempDir()
	checksums := map[string]string{"a.go": "111", "b.go": "222"}
	require.NoError(t, repo.Save(dir, checksums))

	// Act — first load populates cache
	firstLoad, exists, err := repo.Load(dir)
	require.NoError(t, err)
	require.True(t, exists)
	require.Len(t, firstLoad, 2)

	// Mutate the returned map; second load must return the original values
	firstLoad["a.go"] = "changed-locally"
	secondLoad, exists, err := repo.Load(dir)

	// Assert
	require.NoError(t, err)
	require.True(t, exists)
	assert.Equal(t, "111", secondLoad["a.go"], "expected cached checksum to remain immutable")
}

func TestDirectoryChecksumRepository_Load_ReloadsWhenDiskChecksumChanges(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := NewDirectoryChecksumRepository()
	dir := t.TempDir()
	require.NoError(t, repo.Save(dir, map[string]string{"a.go": "111"}))
	_, _, err := repo.Load(dir)
	require.NoError(t, err)

	// Write updated content directly to disk
	updated := checksumPayload{Files: map[string]string{"a.go": "999", "c.go": "333"}}
	content, err := json.Marshal(updated)
	require.NoError(t, err)
	checksumPath := filepath.Join(dir, ".idx", "checksum.idx")
	require.NoError(t, os.WriteFile(checksumPath, content, 0600))

	// Act
	reloaded, exists, err := repo.Load(dir)

	// Assert
	require.NoError(t, err)
	require.True(t, exists)
	assert.Equal(t, "999", reloaded["a.go"])
	assert.Equal(t, "333", reloaded["c.go"])
}

func TestDirectoryChecksumRepository_Load_ClearsCacheWhenFileRemoved(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := NewDirectoryChecksumRepository()
	dir := t.TempDir()
	require.NoError(t, repo.Save(dir, map[string]string{"a.go": "111"}))
	_, _, err := repo.Load(dir)
	require.NoError(t, err)
	checksumPath := filepath.Join(dir, ".idx", "checksum.idx")
	require.NoError(t, os.Remove(checksumPath))

	// Act
	loaded, exists, err := repo.Load(dir)

	// Assert
	require.NoError(t, err)
	assert.False(t, exists)
	assert.Empty(t, loaded)
}

func TestDirectoryChecksumRepository_SaveAndLoadSnapshot_PreservesMetadata(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := NewDirectoryChecksumRepository()
	dir := t.TempDir()
	snapshot := indexing.DirectoryChecksumSnapshot{Files: map[string]indexing.FileChecksumState{
		"a.go": {Checksum: "111", Size: 10, ModTimeUnixNano: 100},
		"b.go": {Checksum: "222", Size: 20, ModTimeUnixNano: 200},
	}}

	// Act
	require.NoError(t, repo.SaveSnapshot(dir, snapshot))
	loaded, exists, err := repo.LoadSnapshot(dir)

	// Assert
	require.NoError(t, err)
	require.True(t, exists)
	assert.Equal(t, int64(10), loaded.Files["a.go"].Size)
	assert.Equal(t, int64(100), loaded.Files["a.go"].ModTimeUnixNano)
	assert.Equal(t, "222", loaded.Files["b.go"].Checksum)
}

func TestDirectoryChecksumRepository_LoadSnapshot_SupportsLegacyPayload(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := NewDirectoryChecksumRepository()
	dir := t.TempDir()
	legacy := checksumPayload{Files: map[string]string{"legacy.go": "abc"}}
	encoded, err := json.Marshal(legacy)
	require.NoError(t, err)

	checksumPath := filepath.Join(dir, ".idx", "checksum.idx")
	require.NoError(t, os.MkdirAll(filepath.Dir(checksumPath), 0750))
	require.NoError(t, os.WriteFile(checksumPath, encoded, 0600))

	// Act
	snapshot, exists, err := repo.LoadSnapshot(dir)

	// Assert
	require.NoError(t, err)
	require.True(t, exists)
	state := snapshot.Files["legacy.go"]
	assert.Equal(t, "abc", state.Checksum)
	assert.Equal(t, int64(0), state.Size)
	assert.Equal(t, int64(0), state.ModTimeUnixNano)
}

func TestDirectoryChecksumRepository_LoadAndSaveSnapshot_ReturnsErrorForInvalidPath(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := NewDirectoryChecksumRepository()

	// Assert — invalid path for load
	_, _, err := repo.LoadSnapshot("\x00invalid")
	require.Error(t, err)

	// Assert — invalid path for save
	err = repo.SaveSnapshot("\x00invalid", indexing.DirectoryChecksumSnapshot{Files: map[string]indexing.FileChecksumState{"a.go": {Checksum: "1"}}})
	require.Error(t, err)
}

func TestPayloadToSnapshot_PrefersFileStatesOverLegacyFiles(t *testing.T) {
	t.Parallel()

	// Arrange
	payload := checksumPayload{
		Files: map[string]string{"legacy.go": "legacy"},
		FileStates: map[string]checksumFileState{
			"modern.go": {Checksum: "modern", Size: 42, ModTimeUnixNano: 77},
		},
	}

	// Act
	snapshot := payloadToSnapshot(payload)

	// Assert
	require.Len(t, snapshot.Files, 1)
	state := snapshot.Files["modern.go"]
	assert.Equal(t, "modern", state.Checksum)
	assert.Equal(t, int64(42), state.Size)
	assert.Equal(t, int64(77), state.ModTimeUnixNano)

}
