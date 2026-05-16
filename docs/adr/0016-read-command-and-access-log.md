# ADR 0016: Read Command and File Access Log

## Status

Accepted

## Context

The `idx search` command ranks files by BM25 score and filename match, but has no signal about which files a user actually reads during a session. Two files with identical BM25 scores cannot be further differentiated by relevance to the user's current workflow.

Additionally, users and AI coding agents working in a project often need to read individual files from the terminal. There was no first-class command for this — the common workaround was `cat`, which has no integration with the idx ecosystem.

Two requirements emerged together:

1. A `read` command that streams a project file safely to stdout.
2. An access log that captures which files are read, to feed a future ranking boost for `idx search`.

## Decision

### `idx read` command

**Streaming via `bufio.Scanner`**
The file is read line by line, never loaded in full into memory. This keeps memory constant regardless of file size and is safe for large files in legacy or generated codebases.

**Project root enforcement**
The service resolves the project root via `.git` traversal and rejects any path (absolute or relative) that resolves outside it. This prevents `idx read ../../etc/passwd`-style escapes without requiring any allowlist.

**Directory rejection**
If the resolved path is a directory, the command returns an error immediately, before any read attempt.

**Line range parameters `--from` / `--to`**
Both are 1-based and inclusive. Zero means unbounded on that end. When `--from` exceeds the total line count, the command exits without output and without error — this is consistent with `head`/`tail` behavior and allows scripted callers to handle large files without pre-checking line counts.

**Private `fileStreamer` interface, not `ports.FileReader`**
The existing `FileReader` port loads the full file into memory. A private `fileStreamer` interface (`OpenFile`, `IsDir`) was introduced so the streaming adapter (`OSFileStreamer`) can be injected without modifying existing ports or their consumers.

### File access log (`.idx/read_log.idx`)

**Location and format**
The log lives at `<project-root>/.idx/read_log.idx`. Each line is:

```
2026-05-16T00:33:35;internal/main.go;3;2993595
```

Fields: `timestamp;relative-path;read-count;inode`. The file is plain text so it is human-readable and grep-able.

**30-day retention**
Entries older than 30 days are pruned on cold cache load. A short TTL keeps the log small in long-lived projects and prevents stale access patterns from permanently biasing rankings.

**Deduplication — one entry per file**
Every call to `RecordRead` increments the `read-count` of the existing entry rather than appending a new line. The log grows at most one line per distinct file, independent of how many times it is read.

**Inode field for rename detection**
When a file is renamed (`mv old.go new.go`), its Unix inode number does not change — only the directory entry changes. By storing the inode at write time, `coldReconcile` can detect that a deleted entry and the new path share the same inode and transfer the accumulated count. Without this, a rename would silently reset the count to 1.

**Deletion pruning**
On cold cache load, `coldReconcile` calls `os.Stat` for each logged path. Entries whose files no longer exist are dropped. This keeps the log from accumulating ghost entries for deleted files indefinitely.

**Cold / warm cache split**
Stat checks and deletion pruning are expensive for large logs. They only happen on a cold cache load (cache TTL: 5 minutes). Subsequent calls within the TTL window call `appendOrIncrement` directly on the in-memory state — no disk reads, no stat calls, only a disk write.

This also means a warm-cache call succeeds even if the log file is temporarily corrupted or locked by another process (only the write path under flock is needed).

**Concurrency safety — two layers**
- `sync.Mutex` serialises goroutines within the same process (e.g. concurrent `idx read` calls from an agent).
- `syscall.Flock(LOCK_EX)` on a companion `.lock` file serialises writes across processes (e.g. two terminal sessions running `idx read` simultaneously).

**System paths excluded from logging**
Files under `.git/` and `.idx/` are never logged. These directories are infrastructure, not project content, and would add noise to the ranking signal without benefit.

**Write-only port (`ReadLogRepository`)**
The port declares only `RecordRead(projectRoot, relativePath string) error`. A `LoadAll` method (for reading the log into the search ranker) is intentionally deferred — the log is collected now, consumed later.

**Log errors are silently swallowed**
`recordReadAccess` ignores errors returned by `RecordRead`. The log is a supplementary signal; a disk-full or permissions error must not prevent the user from reading their file.

**`.idx/` added to `.gitignore` by `idx init`**
The log is local and machine-specific — commit histories should not contain it. `idx init` writes `.idx/` to `.gitignore` automatically.

## Consequences

- `idx read` is a safe, memory-constant file reader that integrates with the idx ranking ecosystem.
- `.idx/read_log.idx` accumulates per-file read counts that can be used as a future boost factor in `idx search` ranking, weighted by recency (timestamp) and frequency (count).
- The log is bounded: at most one entry per project file, entries expire after 30 days, deleted and renamed files are handled correctly.
- Rename detection requires that the rename and the next `idx read` of the new path happen within the same cold-load window (i.e. the old entry must still be in the log). Files renamed after their log entry expired will start with count 1.
- The 5-minute write cache accepts a window of cross-process count divergence: two processes writing to the same log within the TTL may each work from stale in-memory state and overwrite each other's counts. This is an acceptable trade-off for a boost signal that does not need exact precision.
