package indexstore_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"idx/internal/adapters/repository/indexstore"
)

func TestReadLogRepositoryRecordReadCreatesEntryOnFirstRead(t *testing.T) {
	root := t.TempDir()
	repo := indexstore.NewReadLogRepository()

	if err := repo.RecordRead(root, "main.go"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	lines := readLogLines(t, root)
	if len(lines) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(lines))
	}

	parts := strings.Split(lines[0], ";")
	if len(parts) != 4 {
		t.Fatalf("expected 4 semicolon-separated fields, got %d in %q", len(parts), lines[0])
	}
	if parts[1] != "main.go" {
		t.Fatalf("expected path 'main.go', got %q", parts[1])
	}
	if parts[2] != "1" {
		t.Fatalf("expected count '1', got %q", parts[2])
	}
}

func TestReadLogRepositoryRecordReadIncrementsCountOnSubsequentReads(t *testing.T) {
	root := t.TempDir()
	repo := indexstore.NewReadLogRepository()

	for range 3 {
		if err := repo.RecordRead(root, "main.go"); err != nil {
			t.Fatalf("expected no error on repeat read, got %v", err)
		}
	}

	lines := readLogLines(t, root)
	if len(lines) != 1 {
		t.Fatalf("expected 1 consolidated entry, got %d", len(lines))
	}

	parts := strings.Split(lines[0], ";")
	if parts[2] != "3" {
		t.Fatalf("expected count '3', got %q", parts[2])
	}
}

func TestReadLogRepositoryRecordReadUpdatesTimestampOnSubsequentRead(t *testing.T) {
	root := t.TempDir()
	repo := indexstore.NewReadLogRepository()

	if err := repo.RecordRead(root, "main.go"); err != nil {
		t.Fatalf("unexpected first read error: %v", err)
	}

	lines1 := readLogLines(t, root)
	ts1 := strings.Split(lines1[0], ";")[0]

	time.Sleep(time.Second)

	if err := repo.RecordRead(root, "main.go"); err != nil {
		t.Fatalf("unexpected second read error: %v", err)
	}

	lines2 := readLogLines(t, root)
	ts2 := strings.Split(lines2[0], ";")[0]

	if ts1 == ts2 {
		t.Fatalf("expected timestamp to be updated, but both reads produced %q", ts1)
	}
}

func TestReadLogRepositoryRecordReadKeepsSeparateEntriesPerPath(t *testing.T) {
	root := t.TempDir()
	repo := indexstore.NewReadLogRepository()

	if err := repo.RecordRead(root, "main.go"); err != nil {
		t.Fatal(err)
	}
	if err := repo.RecordRead(root, "internal/foo.go"); err != nil {
		t.Fatal(err)
	}

	lines := readLogLines(t, root)
	if len(lines) != 2 {
		t.Fatalf("expected 2 entries for 2 distinct paths, got %d", len(lines))
	}
}

func TestReadLogRepositoryRecordReadPrunesExpiredEntries(t *testing.T) {
	root := t.TempDir()
	logPath := filepath.Join(root, ".idx", "read_log.idx")
	if err := os.MkdirAll(filepath.Dir(logPath), 0750); err != nil {
		t.Fatal(err)
	}

	// Write an entry with a timestamp 31 days in the past.
	expired := time.Now().UTC().Add(-31 * 24 * time.Hour).Format("2006-01-02T15:04:05")
	staleEntry := expired + ";old_file.go;5\n"
	if err := os.WriteFile(logPath, []byte(staleEntry), 0600); err != nil {
		t.Fatal(err)
	}

	repo := indexstore.NewReadLogRepository()
	if err := repo.RecordRead(root, "new_file.go"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	lines := readLogLines(t, root)
	if len(lines) != 1 {
		t.Fatalf("expected only 1 live entry after pruning expired, got %d: %v", len(lines), lines)
	}
	if strings.Contains(lines[0], "old_file.go") {
		t.Fatalf("expected expired entry to be pruned, but found it: %q", lines[0])
	}
}

func TestReadLogRepositoryRecordReadPreservesUnexpiredEntriesForOtherPaths(t *testing.T) {
	root := t.TempDir()
	logPath := filepath.Join(root, ".idx", "read_log.idx")
	if err := os.MkdirAll(filepath.Dir(logPath), 0750); err != nil {
		t.Fatal(err)
	}

	// Create the file on disk so deletion pruning does not remove the entry.
	if err := os.WriteFile(filepath.Join(root, "recent_file.go"), []byte("package p\n"), 0600); err != nil {
		t.Fatal(err)
	}

	recent := time.Now().UTC().Add(-5 * 24 * time.Hour).Format("2006-01-02T15:04:05")
	recentEntry := recent + ";recent_file.go;3\n"
	if err := os.WriteFile(logPath, []byte(recentEntry), 0600); err != nil {
		t.Fatal(err)
	}

	repo := indexstore.NewReadLogRepository()
	if err := repo.RecordRead(root, "another.go"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	lines := readLogLines(t, root)
	if len(lines) != 2 {
		t.Fatalf("expected 2 entries (recent preserved + new), got %d: %v", len(lines), lines)
	}
}

func TestReadLogRepositoryRecordReadCreatesIdxDirectoryIfMissing(t *testing.T) {
	root := t.TempDir()
	// Do NOT pre-create .idx/ — RecordRead must create it.
	repo := indexstore.NewReadLogRepository()

	if err := repo.RecordRead(root, "main.go"); err != nil {
		t.Fatalf("expected RecordRead to create .idx dir, got error: %v", err)
	}

	lines := readLogLines(t, root)
	if len(lines) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(lines))
	}
}

func TestReadLogRepositoryRecordReadSkipsMalformedLines(t *testing.T) {
	root := t.TempDir()
	logPath := filepath.Join(root, ".idx", "read_log.idx")
	if err := os.MkdirAll(filepath.Dir(logPath), 0750); err != nil {
		t.Fatal(err)
	}

	// Mix malformed and valid lines.
	recent := time.Now().UTC().Format("2006-01-02T15:04:05")
	content := "not-a-valid-entry\n" +
		recent + ";good_file.go;2\n" +
		"also-bad\n"
	if err := os.WriteFile(logPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	repo := indexstore.NewReadLogRepository()
	if err := repo.RecordRead(root, "another.go"); err != nil {
		t.Fatalf("expected no error when skipping malformed lines, got %v", err)
	}

	lines := readLogLines(t, root)
	for _, line := range lines {
		if strings.Contains(line, "not-a-valid-entry") || strings.Contains(line, "also-bad") {
			t.Fatalf("expected malformed lines to be dropped, found: %q", line)
		}
	}
}

func TestReadLogRepositoryRecordReadFormatsTimestampWithoutTimezone(t *testing.T) {
	root := t.TempDir()
	repo := indexstore.NewReadLogRepository()

	if err := repo.RecordRead(root, "main.go"); err != nil {
		t.Fatal(err)
	}

	lines := readLogLines(t, root)
	ts := strings.Split(lines[0], ";")[0]

	// Should match "2006-01-02T15:04:05" — no timezone suffix.
	if strings.Contains(ts, "Z") || strings.Contains(ts, "+") {
		t.Fatalf("expected timestamp without timezone, got %q", ts)
	}
	if len(ts) != 19 {
		t.Fatalf("expected timestamp length 19, got %d in %q", len(ts), ts)
	}
}

func TestReadLogRepositoryRecordReadConcurrentGoroutinesProduceExactCount(t *testing.T) {
	root := t.TempDir()
	repo := indexstore.NewReadLogRepository()

	const workers = 20
	var wg sync.WaitGroup
	wg.Add(workers)

	for range workers {
		go func() {
			defer wg.Done()
			if err := repo.RecordRead(root, "main.go"); err != nil {
				t.Errorf("unexpected RecordRead error: %v", err)
			}
		}()
	}
	wg.Wait()

	lines := readLogLines(t, root)
	if len(lines) != 1 {
		t.Fatalf("expected 1 consolidated entry, got %d: %v", len(lines), lines)
	}

	parts := strings.Split(lines[0], ";")
	if len(parts) != 4 {
		t.Fatalf("malformed log entry: %q", lines[0])
	}

	count, err := strconv.Atoi(parts[2])
	if err != nil {
		t.Fatalf("could not parse count from %q: %v", lines[0], err)
	}
	if count != workers {
		t.Fatalf("expected count %d from %d concurrent reads, got %d", workers, workers, count)
	}
}

func TestReadLogRepositoryRecordReadConcurrentGoroutinesMultiplePaths(t *testing.T) {
	root := t.TempDir()
	repo := indexstore.NewReadLogRepository()

	paths := []string{"a.go", "b.go", "c.go"}
	const readsPerPath = 5
	var wg sync.WaitGroup

	for _, p := range paths {
		for range readsPerPath {
			wg.Add(1)
			go func(path string) {
				defer wg.Done()
				if err := repo.RecordRead(root, path); err != nil {
					t.Errorf("unexpected error for %q: %v", path, err)
				}
			}(p)
		}
	}
	wg.Wait()

	lines := readLogLines(t, root)
	if len(lines) != len(paths) {
		t.Fatalf("expected %d entries (one per path), got %d: %v", len(paths), len(lines), lines)
	}

	for _, line := range lines {
		parts := strings.Split(line, ";")
		count, err := strconv.Atoi(parts[2])
		if err != nil {
			t.Fatalf("could not parse count from %q", line)
		}
		if count != readsPerPath {
			t.Fatalf("expected count %d for %q, got %d", readsPerPath, parts[1], count)
		}
	}
}

func TestReadLogRepositoryRecordReadUsesWriteCacheOnSubsequentCalls(t *testing.T) {
	root := t.TempDir()
	repo := indexstore.NewReadLogRepository()

	if err := repo.RecordRead(root, "main.go"); err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	// Corrupt the log file on disk — if the cache is cold the second call
	// would parse invalid data and may return an error or reset the count.
	logPath := filepath.Join(root, ".idx", "read_log.idx")
	if err := os.WriteFile(logPath, []byte("not-valid-data\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// Second call must succeed using the warm write cache (skips the corrupt file).
	if err := repo.RecordRead(root, "main.go"); err != nil {
		t.Fatalf("expected warm-cache call to succeed, got %v", err)
	}

	// After the second call the log is rewritten from the in-memory state, so
	// the count must be 2 (not reset to 1 or errored due to corruption).
	lines := readLogLines(t, root)
	if len(lines) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(lines))
	}
	parts := strings.Split(lines[0], ";")
	if parts[2] != "2" {
		t.Fatalf("expected count '2' from warm cache, got %q", parts[2])
	}
}

func TestReadLogRepositoryRecordReadPrunesDeletedFileEntries(t *testing.T) {
	root := t.TempDir()

	// Create the file on disk and record a read so the repo has an entry.
	filePath := filepath.Join(root, "will_be_deleted.go")
	if err := os.WriteFile(filePath, []byte("package p\n"), 0600); err != nil {
		t.Fatal(err)
	}

	repo := indexstore.NewReadLogRepository()
	if err := repo.RecordRead(root, "will_be_deleted.go"); err != nil {
		t.Fatalf("first RecordRead failed: %v", err)
	}

	// Remove the file and force a cache miss so reconcileAndUpsert runs from disk.
	if err := os.Remove(filePath); err != nil {
		t.Fatal(err)
	}

	// Use a fresh repo so the write cache is cold.
	repo2 := indexstore.NewReadLogRepository()
	if err := repo2.RecordRead(root, "new_file.go"); err != nil {
		t.Fatalf("second RecordRead failed: %v", err)
	}

	lines := readLogLines(t, root)
	for _, line := range lines {
		if strings.Contains(line, "will_be_deleted.go") {
			t.Fatalf("expected deleted file entry to be pruned, found: %q", line)
		}
	}
}

func TestReadLogRepositoryRecordReadTransfersCountOnFileRename(t *testing.T) {
	root := t.TempDir()

	// Create a file and record reads so the entry has count > 1.
	oldPath := filepath.Join(root, "old_name.go")
	if err := os.WriteFile(oldPath, []byte("package p\n"), 0600); err != nil {
		t.Fatal(err)
	}

	repo := indexstore.NewReadLogRepository()
	for range 3 {
		if err := repo.RecordRead(root, "old_name.go"); err != nil {
			t.Fatalf("RecordRead on old name failed: %v", err)
		}
	}

	// Rename the file (same inode, new path).
	newPath := filepath.Join(root, "new_name.go")
	if err := os.Rename(oldPath, newPath); err != nil {
		t.Fatal(err)
	}

	// Use a fresh repo so the write cache is cold and reconciliation runs fresh.
	repo2 := indexstore.NewReadLogRepository()
	if err := repo2.RecordRead(root, "new_name.go"); err != nil {
		t.Fatalf("RecordRead on new name failed: %v", err)
	}

	lines := readLogLines(t, root)
	for _, line := range lines {
		if strings.Contains(line, "new_name.go") {
			parts := strings.Split(line, ";")
			count, err := strconv.Atoi(parts[2])
			if err != nil {
				t.Fatalf("could not parse count from %q", line)
			}
			// 3 prior reads + 1 new read = 4
			if count != 4 {
				t.Fatalf("expected inherited count 4 after rename, got %d", count)
			}
			return
		}
	}
	t.Fatal("expected new_name.go entry in log after rename, found none")
}

// readLogLines reads and returns non-empty lines from the read log file.
func readLogLines(t *testing.T, root string) []string {
	t.Helper()
	logPath := filepath.Join(root, ".idx", "read_log.idx")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("expected log file to exist, got error: %v", err)
	}

	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, strings.TrimSpace(line))
		}
	}
	return lines
}
