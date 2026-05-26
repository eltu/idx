package read

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	readLogFileName  = "read_log.idx"
	readLogSeparator = ";"
	readLogRetention = 30 * 24 * time.Hour
	readLogCacheTTL  = 5 * time.Minute
)

type readLogEntry struct {
	updatedAt time.Time
	path      string
	readCount int
	// inode is the Unix inode number at the time of the last RecordRead.
	// It is used to detect file renames: same inode + different path = renamed file.
	inode uint64
}

// ReadLogRepository persists per-file access history to .idx/read_log.idx.
//
// Concurrency model:
//   - sync.Mutex serializes goroutines within the same process.
//   - syscall.Flock (advisory, exclusive) serializes disk writes across processes.
//
// Caching:
//   - writeState is the in-memory working copy for the write path (RecordRead).
//     It avoids disk reads on every write while the TTL is live.
//   - readSnap is reserved for the future read path (LoadAll for search boost)
//     and is intentionally independent of writeState.
type ReadLogRepository struct {
	mu sync.Mutex

	writeState    []readLogEntry
	writeStateExp time.Time

	// readSnap and readSnapExp are populated when LoadAll is implemented.
	// They are kept separate from writeState so read and write TTLs are independent.
	readSnap    []readLogEntry
	readSnapExp time.Time
}

// NewReadLogRepository creates a read log repository backed by the local filesystem.
// Example: repo := NewReadLogRepository().
func NewReadLogRepository() *ReadLogRepository {
	return &ReadLogRepository{}
}

// RecordRead upserts the access entry for relativePath: refreshes the timestamp and
// increments the read count. On cache miss it also:
//   - prunes entries whose files no longer exist on disk,
//   - detects renames via inode comparison and transfers the count to the new path,
//   - drops entries older than 30 days.
//
// The write state is cached for readLogCacheTTL to avoid disk reads on every call.
// Disk writes are protected by an exclusive flock so concurrent processes do not
// produce partial writes.
func (r *ReadLogRepository) RecordRead(projectRoot, relativePath string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	inode := readFileInode(filepath.Join(projectRoot, relativePath))

	if now.After(r.writeStateExp) {
		entries, err := loadReadLogEntries(readLogPath(projectRoot))
		if err != nil {
			return err
		}
		// Full reconciliation on cold load: prune expired/deleted, detect renames.
		r.writeState = coldReconcile(entries, relativePath, inode, projectRoot, now)
	} else {
		// Warm cache: just increment or append — no disk reads or stat checks.
		r.writeState = appendOrIncrement(r.writeState, relativePath, inode, 0, now)
	}

	logPath := readLogPath(projectRoot)
	if err := r.withFileLock(logPath+".lock", func() error {
		return saveReadLogEntries(logPath, r.writeState)
	}); err != nil {
		return err
	}

	r.writeStateExp = now.Add(readLogCacheTTL)
	return nil
}

// LoadAll returns a snapshot of all current read log entries for projectRoot.
// Results are cached for readLogCacheTTL to avoid repeated disk reads during a
// search session. Returns an empty slice (not an error) if the log does not exist.
func (r *ReadLogRepository) LoadAll(projectRoot string) ([]LogEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if time.Now().Before(r.readSnapExp) {
		return toPortEntries(r.readSnap), nil
	}

	entries, err := loadReadLogEntries(readLogPath(projectRoot))
	if err != nil {
		return nil, err
	}

	r.readSnap = entries
	r.readSnapExp = time.Now().Add(readLogCacheTTL)
	return toPortEntries(entries), nil
}

func toPortEntries(entries []readLogEntry) []LogEntry {
	out := make([]LogEntry, len(entries))
	for i, e := range entries {
		out[i] = LogEntry{Path: e.path, ReadCount: e.readCount, LastReadAt: e.updatedAt}
	}
	return out
}

// coldReconcile is called on cache miss. It prunes expired and deleted entries,
// detects file renames via inode, then upserts the current access.
func coldReconcile(entries []readLogEntry, relativePath string, inode uint64, projectRoot string, now time.Time) []readLogEntry {
	cutoff := now.Add(-readLogRetention)
	live := make([]readLogEntry, 0, len(entries)+1)
	inherited := 0

	for _, e := range entries {
		if e.updatedAt.Before(cutoff) {
			continue
		}
		absPath := filepath.Join(projectRoot, e.path)
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			// File was deleted. If it was renamed to relativePath, inherit its count.
			if inode != 0 && e.inode == inode && e.path != relativePath {
				inherited = e.readCount
			}
			continue
		}
		live = append(live, e)
	}

	return appendOrIncrement(live, relativePath, inode, inherited, now)
}

// appendOrIncrement finds an existing entry for relativePath and increments it,
// or appends a new entry starting at inherited+1.
func appendOrIncrement(entries []readLogEntry, relativePath string, inode uint64, inherited int, now time.Time) []readLogEntry {
	for i, e := range entries {
		if e.path == relativePath {
			entries[i].updatedAt = now
			entries[i].readCount++
			entries[i].inode = inode
			return entries
		}
	}
	return append(entries, readLogEntry{
		updatedAt: now,
		path:      relativePath,
		readCount: inherited + 1,
		inode:     inode,
	})
}

// withFileLock creates (or opens) the lock file, acquires an exclusive flock,
// runs fn, then releases the lock. The .idx directory is created if missing.
func (r *ReadLogRepository) withFileLock(lockPath string, fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(lockPath), 0750); err != nil {
		return fmt.Errorf("failed to create log directory %q: got error %v, expected a writable path", filepath.Dir(lockPath), err)
	}

	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600) //nolint:gosec
	if err != nil {
		return fmt.Errorf("failed to create lock file %q: got error %v, expected a writable path", lockPath, err)
	}
	defer func() { _ = f.Close() }()

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil { //nolint:gosec // fd value fits int on all target platforms
		return fmt.Errorf("failed to acquire file lock on %q: got error %v", lockPath, err)
	}
	defer func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }() //nolint:gosec // fd value fits int on all target platforms

	return fn()
}

// readFileInode returns the Unix inode number for path, or 0 if unavailable.
func readFileInode(path string) uint64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0
	}
	return stat.Ino
}

func readLogPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".idx", readLogFileName)
}

func loadReadLogEntries(logPath string) ([]readLogEntry, error) {
	f, err := os.Open(logPath) //nolint:gosec
	if os.IsNotExist(err) {
		return []readLogEntry{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to open read log %q: got error %v, expected a readable file", logPath, err)
	}
	defer func() { _ = f.Close() }()

	var entries []readLogEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		entry, err := parseReadLogEntry(line)
		if err != nil {
			continue // skip malformed lines
		}
		entries = append(entries, entry)
	}

	return entries, scanner.Err()
}

// parseReadLogEntry handles both the legacy 3-field format (date;path;count)
// and the current 4-field format (date;path;count;inode).
func parseReadLogEntry(line string) (readLogEntry, error) {
	parts := strings.SplitN(line, readLogSeparator, 4)
	if len(parts) < 3 {
		return readLogEntry{}, fmt.Errorf("invalid read log entry %q: expected date;path;count[;inode]", line)
	}

	t, err := time.Parse("2006-01-02T15:04:05", parts[0])
	if err != nil {
		return readLogEntry{}, fmt.Errorf("invalid timestamp in read log entry %q: got %q", line, parts[0])
	}

	count, err := strconv.Atoi(parts[2])
	if err != nil || count < 1 {
		return readLogEntry{}, fmt.Errorf("invalid count in read log entry %q: got %q", line, parts[2])
	}

	var inode uint64
	if len(parts) == 4 {
		inode, _ = strconv.ParseUint(parts[3], 10, 64)
	}

	return readLogEntry{updatedAt: t.UTC(), path: parts[1], readCount: count, inode: inode}, nil
}

func saveReadLogEntries(logPath string, entries []readLogEntry) error {
	if err := os.MkdirAll(filepath.Dir(logPath), 0750); err != nil {
		return fmt.Errorf("failed to create log directory %q: got error %v, expected a writable path", filepath.Dir(logPath), err)
	}

	var sb strings.Builder
	for _, e := range entries {
		sb.WriteString(formatReadLogEntry(e))
		sb.WriteByte('\n')
	}

	if err := os.WriteFile(logPath, []byte(sb.String()), 0600); err != nil {
		return fmt.Errorf("failed to write read log %q: got error %v, expected a writable path", logPath, err)
	}

	return nil
}

func formatReadLogEntry(e readLogEntry) string {
	return fmt.Sprintf("%s;%s;%d;%d",
		e.updatedAt.UTC().Format("2006-01-02T15:04:05"),
		e.path,
		e.readCount,
		e.inode,
	)
}
