package read

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestMatrixRepo() *CoReadMatrixRepository {
	return NewCoReadMatrixRepository()
}

func TestCoReadMatrixRepository_RecordCoRead_SingleRead_NoCoOccurrence(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	repo := newTestMatrixRepo()

	// Act: record one file — no session partner yet.
	err := repo.RecordCoRead(root, "a.go")

	// Assert
	require.NoError(t, err)
	counts, err := repo.LoadCoReads(root, "a.go")
	require.NoError(t, err)
	assert.Empty(t, counts)
}

func TestCoReadMatrixRepository_RecordCoRead_TwoFilesInSession_UpdatesBothDirections(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	repo := newTestMatrixRepo()

	// Act: read a.go then b.go in the same session.
	require.NoError(t, repo.RecordCoRead(root, "a.go"))
	require.NoError(t, repo.RecordCoRead(root, "b.go"))

	// Assert: a→b and b→a should each have count 1.
	countsA, err := repo.LoadCoReads(root, "a.go")
	require.NoError(t, err)
	assert.Equal(t, uint32(1), countsA["b.go"])

	countsB, err := repo.LoadCoReads(root, "b.go")
	require.NoError(t, err)
	assert.Equal(t, uint32(1), countsB["a.go"])
}

func TestCoReadMatrixRepository_RecordCoRead_RepeatedSessions_AccumulatesCounts(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	repo := newTestMatrixRepo()

	// Act: simulate two sessions (clear session between them by using a new repo
	// but pointing at the same disk path, so the matrix is loaded from disk).
	require.NoError(t, repo.RecordCoRead(root, "a.go"))
	require.NoError(t, repo.RecordCoRead(root, "b.go"))

	repo2 := newTestMatrixRepo()
	require.NoError(t, repo2.RecordCoRead(root, "a.go"))
	require.NoError(t, repo2.RecordCoRead(root, "b.go"))

	// Assert: count should be 2 now (once per session).
	counts, err := repo2.LoadCoReads(root, "a.go")
	require.NoError(t, err)
	assert.Equal(t, uint32(2), counts["b.go"])
}

func TestCoReadMatrixRepository_RecordCoRead_ThreeFilesInSession_AllPairsUpdated(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	repo := newTestMatrixRepo()

	// Act
	require.NoError(t, repo.RecordCoRead(root, "a.go"))
	require.NoError(t, repo.RecordCoRead(root, "b.go"))
	require.NoError(t, repo.RecordCoRead(root, "c.go"))

	// Assert: a-b, a-c, b-c pairs all updated.
	countsA, err := repo.LoadCoReads(root, "a.go")
	require.NoError(t, err)
	assert.Equal(t, uint32(1), countsA["b.go"])
	assert.Equal(t, uint32(1), countsA["c.go"])

	countsB, err := repo.LoadCoReads(root, "b.go")
	require.NoError(t, err)
	assert.Equal(t, uint32(1), countsB["c.go"])
}

func TestCoReadMatrixRepository_LoadCoReads_UnknownFile_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	repo := newTestMatrixRepo()

	// Act
	counts, err := repo.LoadCoReads(root, "unknown.go")

	// Assert
	require.NoError(t, err)
	assert.Empty(t, counts)
}

func TestCoReadMatrixRepository_LoadCoReads_ReturnsClonesNotReference(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	repo := newTestMatrixRepo()
	require.NoError(t, repo.RecordCoRead(root, "a.go"))
	require.NoError(t, repo.RecordCoRead(root, "b.go"))

	// Act: mutate the returned map.
	counts, err := repo.LoadCoReads(root, "a.go")
	require.NoError(t, err)
	counts["b.go"] = 999

	// Assert: subsequent load still returns original value.
	counts2, err := repo.LoadCoReads(root, "a.go")
	require.NoError(t, err)
	assert.Equal(t, uint32(1), counts2["b.go"])
}

func TestCoReadMatrixRepository_PurgeSession_StaleEntryExcluded(t *testing.T) {
	t.Parallel()

	// Arrange: seed the session with a stale entry by manually backdating it.
	root := t.TempDir()
	repo := newTestMatrixRepo()
	repo.sessionReads["old.go"] = time.Now().Add(-coReadSessionWindow - time.Second)

	// Act: record new file — old.go should be purged before pairing.
	require.NoError(t, repo.RecordCoRead(root, "new.go"))

	// Assert: new.go has no co-occurrence with old.go.
	counts, err := repo.LoadCoReads(root, "new.go")
	require.NoError(t, err)
	assert.Empty(t, counts)
}

func TestCoReadMatrixRepository_Persistence_SurvivesNewInstance(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	repo := newTestMatrixRepo()
	require.NoError(t, repo.RecordCoRead(root, "a.go"))
	require.NoError(t, repo.RecordCoRead(root, "b.go"))

	// Act: load with a fresh instance (no in-memory cache).
	fresh := newTestMatrixRepo()
	counts, err := fresh.LoadCoReads(root, "a.go")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, uint32(1), counts["b.go"])
}

func TestCoReadMatrixRepository_MatrixFilePath_UnderIdxDir(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	expected := filepath.Join(root, ".idx", coReadMatrixFileName)
	assert.Equal(t, expected, coReadMatrixPath(root))
}
