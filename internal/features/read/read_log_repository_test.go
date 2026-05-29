package read_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"idx/internal/features/read"
)

func TestReadLogRepository_RecordRead_CreatesEntryOnFirstRead(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	repo := read.NewReadLogRepository()

	// Act
	require.NoError(t, repo.RecordRead(root, "main.go"))

	// Assert
	lines := readLogLines(t, root)
	require.Len(t, lines, 1)
	parts := strings.Split(lines[0], ";")
	require.Len(t, parts, 4)
	assert.Equal(t, "main.go", parts[1])
	assert.Equal(t, "1", parts[2])
}

func TestReadLogRepository_RecordRead_IncrementsCountOnSubsequentReads(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	repo := read.NewReadLogRepository()

	// Act
	for range 3 {
		require.NoError(t, repo.RecordRead(root, "main.go"))
	}

	// Assert
	lines := readLogLines(t, root)
	require.Len(t, lines, 1)
	parts := strings.Split(lines[0], ";")
	assert.Equal(t, "3", parts[2])
}

func TestReadLogRepository_RecordRead_UpdatesTimestampOnSubsequentRead(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	repo := read.NewReadLogRepository()
	require.NoError(t, repo.RecordRead(root, "main.go"))
	ts1 := strings.Split(readLogLines(t, root)[0], ";")[0]

	// Sleep for 1s because the log uses second-precision timestamps ("2006-01-02T15:04:05");
	// wall-clock must advance by at least one second for the update to be observable.
	time.Sleep(time.Second)

	// Act
	require.NoError(t, repo.RecordRead(root, "main.go"))

	// Assert
	ts2 := strings.Split(readLogLines(t, root)[0], ";")[0]
	assert.NotEqual(t, ts1, ts2, "expected timestamp to be updated on subsequent read")
}

func TestReadLogRepository_RecordRead_KeepsSeparateEntriesPerPath(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	repo := read.NewReadLogRepository()

	// Act
	require.NoError(t, repo.RecordRead(root, "main.go"))
	require.NoError(t, repo.RecordRead(root, "internal/foo.go"))

	// Assert
	assert.Len(t, readLogLines(t, root), 2)
}

func TestReadLogRepository_RecordRead_PrunesExpiredEntries(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	logPath := filepath.Join(root, ".idx", "read_log.idx")
	require.NoError(t, os.MkdirAll(filepath.Dir(logPath), 0750))

	// Write an entry with a timestamp 31 days in the past.
	expired := time.Now().UTC().Add(-31 * 24 * time.Hour).Format("2006-01-02T15:04:05")
	require.NoError(t, os.WriteFile(logPath, []byte(expired+";old_file.go;5\n"), 0600))

	repo := read.NewReadLogRepository()

	// Act
	require.NoError(t, repo.RecordRead(root, "new_file.go"))

	// Assert
	lines := readLogLines(t, root)
	require.Len(t, lines, 1, "expected only 1 live entry after pruning expired")
	assert.NotContains(t, lines[0], "old_file.go")
}

func TestReadLogRepository_RecordRead_PreservesUnexpiredEntriesForOtherPaths(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	logPath := filepath.Join(root, ".idx", "read_log.idx")
	require.NoError(t, os.MkdirAll(filepath.Dir(logPath), 0750))

	// Create the file on disk so deletion pruning does not remove the entry.
	require.NoError(t, os.WriteFile(filepath.Join(root, "recent_file.go"), []byte("package p\n"), 0600))
	recent := time.Now().UTC().Add(-5 * 24 * time.Hour).Format("2006-01-02T15:04:05")
	require.NoError(t, os.WriteFile(logPath, []byte(recent+";recent_file.go;3\n"), 0600))

	repo := read.NewReadLogRepository()

	// Act
	require.NoError(t, repo.RecordRead(root, "another.go"))

	// Assert
	assert.Len(t, readLogLines(t, root), 2, "expected recent entry preserved plus new entry")
}

func TestReadLogRepository_RecordRead_CreatesIdxDirectoryIfMissing(t *testing.T) {
	t.Parallel()

	// Arrange — do NOT pre-create .idx/ — RecordRead must create it
	root := t.TempDir()
	repo := read.NewReadLogRepository()

	// Act
	require.NoError(t, repo.RecordRead(root, "main.go"))

	// Assert
	assert.Len(t, readLogLines(t, root), 1)
}

func TestReadLogRepository_RecordRead_SkipsMalformedLines(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	logPath := filepath.Join(root, ".idx", "read_log.idx")
	require.NoError(t, os.MkdirAll(filepath.Dir(logPath), 0750))
	recent := time.Now().UTC().Format("2006-01-02T15:04:05")
	content := "not-a-valid-entry\n" + recent + ";good_file.go;2\n" + "also-bad\n"
	require.NoError(t, os.WriteFile(logPath, []byte(content), 0600))

	repo := read.NewReadLogRepository()

	// Act
	require.NoError(t, repo.RecordRead(root, "another.go"))

	// Assert
	for _, line := range readLogLines(t, root) {
		assert.NotContains(t, line, "not-a-valid-entry")
		assert.NotContains(t, line, "also-bad")
	}
}

func TestReadLogRepository_RecordRead_FormatsTimestampWithoutTimezone(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	repo := read.NewReadLogRepository()

	// Act
	require.NoError(t, repo.RecordRead(root, "main.go"))

	// Assert — should match "2006-01-02T15:04:05" — no timezone suffix
	ts := strings.Split(readLogLines(t, root)[0], ";")[0]
	assert.NotContains(t, ts, "Z")
	assert.NotContains(t, ts, "+")
	assert.Len(t, ts, 19)
}

func TestReadLogRepository_RecordRead_ConcurrentGoroutinesProduceExactCount(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	repo := read.NewReadLogRepository()

	const workers = 20
	var wg sync.WaitGroup
	wg.Add(workers)

	// Act
	for range workers {
		go func() {
			defer wg.Done()
			assert.NoError(t, repo.RecordRead(root, "main.go"))
		}()
	}
	wg.Wait()

	// Assert
	lines := readLogLines(t, root)
	require.Len(t, lines, 1)
	parts := strings.Split(lines[0], ";")
	require.Len(t, parts, 4)
	count, err := strconv.Atoi(parts[2])
	require.NoError(t, err)
	assert.Equal(t, workers, count)
}

func TestReadLogRepository_RecordRead_ConcurrentGoroutinesMultiplePaths(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	repo := read.NewReadLogRepository()

	paths := []string{"a.go", "b.go", "c.go"}
	const readsPerPath = 5
	var wg sync.WaitGroup

	// Act
	for _, p := range paths {
		for range readsPerPath {
			wg.Add(1)
			go func(path string) {
				defer wg.Done()
				assert.NoError(t, repo.RecordRead(root, path))
			}(p)
		}
	}
	wg.Wait()

	// Assert
	lines := readLogLines(t, root)
	require.Len(t, lines, len(paths))
	for _, line := range lines {
		parts := strings.Split(line, ";")
		count, err := strconv.Atoi(parts[2])
		require.NoError(t, err)
		assert.Equal(t, readsPerPath, count, "expected count %d for %q", readsPerPath, parts[1])
	}
}

func TestReadLogRepository_RecordRead_UsesWriteCacheOnSubsequentCalls(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	repo := read.NewReadLogRepository()
	require.NoError(t, repo.RecordRead(root, "main.go"))

	// Corrupt the log file on disk — if the cache is cold the second call
	// would parse invalid data and may return an error or reset the count.
	logPath := filepath.Join(root, ".idx", "read_log.idx")
	require.NoError(t, os.WriteFile(logPath, []byte("not-valid-data\n"), 0600))

	// Act — second call must succeed using the warm write cache (skips the corrupt file)
	require.NoError(t, repo.RecordRead(root, "main.go"))

	// Assert — after the second call the log is rewritten from in-memory state, count must be 2
	lines := readLogLines(t, root)
	require.Len(t, lines, 1)
	parts := strings.Split(lines[0], ";")
	assert.Equal(t, "2", parts[2])
}

func TestReadLogRepository_RecordRead_PrunesDeletedFileEntries(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	filePath := filepath.Join(root, "will_be_deleted.go")
	require.NoError(t, os.WriteFile(filePath, []byte("package p\n"), 0600))

	repo := read.NewReadLogRepository()
	require.NoError(t, repo.RecordRead(root, "will_be_deleted.go"))

	// Remove the file and force a cache miss so reconcileAndUpsert runs from disk.
	require.NoError(t, os.Remove(filePath))

	// Act — use a fresh repo so the write cache is cold
	repo2 := read.NewReadLogRepository()
	require.NoError(t, repo2.RecordRead(root, "new_file.go"))

	// Assert
	for _, line := range readLogLines(t, root) {
		assert.NotContains(t, line, "will_be_deleted.go")
	}
}

func TestReadLogRepository_RecordRead_TransfersCountOnFileRename(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	oldPath := filepath.Join(root, "old_name.go")
	require.NoError(t, os.WriteFile(oldPath, []byte("package p\n"), 0600))

	repo := read.NewReadLogRepository()
	for range 3 {
		require.NoError(t, repo.RecordRead(root, "old_name.go"))
	}

	// Rename the file (same inode, new path).
	newPath := filepath.Join(root, "new_name.go")
	require.NoError(t, os.Rename(oldPath, newPath))

	// Act — use a fresh repo so the write cache is cold and reconciliation runs fresh
	repo2 := read.NewReadLogRepository()
	require.NoError(t, repo2.RecordRead(root, "new_name.go"))

	// Assert — 3 prior reads + 1 new read = 4
	for _, line := range readLogLines(t, root) {
		if strings.Contains(line, "new_name.go") {
			parts := strings.Split(line, ";")
			count, err := strconv.Atoi(parts[2])
			require.NoError(t, err)
			assert.Equal(t, 4, count, "expected inherited count 4 after rename")
			return
		}
	}
	t.Fatal("expected new_name.go entry in log after rename, found none")
}

func TestReadLogRepository_LoadAll_EmptyWhenNoFile(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	repo := read.NewReadLogRepository()

	// Act
	entries, err := repo.LoadAll(root)

	// Assert
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestReadLogRepository_LoadAll_ReturnsRecordedEntries(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	repo := read.NewReadLogRepository()
	require.NoError(t, repo.RecordRead(root, "cmd/main.go"))
	require.NoError(t, repo.RecordRead(root, "cmd/main.go"))
	require.NoError(t, repo.RecordRead(root, "go.mod"))

	// Act — use a fresh repo to bypass the write cache and read from disk
	freshRepo := read.NewReadLogRepository()
	entries, err := freshRepo.LoadAll(root)

	// Assert
	require.NoError(t, err)
	require.Len(t, entries, 2)
	byPath := map[string]int{}
	for _, e := range entries {
		byPath[e.Path] = e.ReadCount
		assert.False(t, e.LastReadAt.IsZero(), "expected non-zero LastReadAt for %q", e.Path)
	}
	assert.Equal(t, 2, byPath["cmd/main.go"])
	assert.Equal(t, 1, byPath["go.mod"])
}

func TestReadLogRepository_LoadAll_UsesCache(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	repo := read.NewReadLogRepository()
	require.NoError(t, repo.RecordRead(root, "main.go"))

	// Act — first call populates the read cache; second should return same result
	first, err := repo.LoadAll(root)
	require.NoError(t, err)
	second, err := repo.LoadAll(root)
	require.NoError(t, err)

	// Assert
	assert.Len(t, second, len(first))
}

// readLogLines reads and returns non-empty lines from the read log file.
func readLogLines(t *testing.T, root string) []string {
	t.Helper()
	logPath := filepath.Join(root, ".idx", "read_log.idx")
	data, err := os.ReadFile(logPath)
	require.NoError(t, err, "expected log file to exist")

	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, strings.TrimSpace(line))
		}
	}
	return lines
}
