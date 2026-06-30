# ADR 0025: `idx related` — Git Co-Change + Persistent Co-Read Matrix

## Status

Accepted — supersedes the co-read signal in ADR 0024.

## Context

The co-read signal introduced in ADR 0024 used a temporal proximity proxy: files read within ±2 hours of the target were treated as co-read. This approach is amnésic — the correlation history is lost when the read log TTL expires (30 days) or when files are accessed in different sessions over time.

Two stronger signals are available:

1. **Git co-change**: files that appear together in the same commits have structural coupling and provide a durable, version-controlled history of change affinity.
2. **Persistent co-read matrix**: an accumulated counter of how many sessions included each file pair, stored across restarts and not subject to TTL decay.

## Decision

### Signal weights

| Signal | Weight | Prior weight (ADR 0024) |
|--------|--------|------------------------|
| Git co-change | 0.5 | — (new) |
| Persistent co-read matrix | 0.3 | 0.7 (temporal proxy) |
| BM25 term co-occurrence | 0.2 | 0.3 |

### Git co-change signal

Uses a two-call git approach to avoid pathspec filtering artifacts:

1. `git log --format=%H -- <relPath>` → list of commit SHAs that touched the target file.
2. `git diff-tree --root -r --name-only sha1 sha2 ...` → all files from those commits.

The `--root` flag ensures root commits (no parent) are included. Score: `commits_together(A,B) / total_commits(A)`, normalized in [0,1]. Errors are silently swallowed (`applyGitCoChange` is a best-effort signal).

### Persistent co-read matrix

**Port**: `internal/shared/coread/MatrixRepository` interface with `RecordCoRead` and `LoadCoReads`.

**Implementation**: `internal/features/read/CoReadMatrixRepository`:
- Maintains in-memory `sessionReads map[string]time.Time` with a 30-minute session window.
- On each `RecordCoRead`: purges stale session entries, increments `matrix[A][B]` and `matrix[B][A]` for every active session pair, appends the current file to the session, and persists to disk.
- Storage: GOB at `.idx/co_read_matrix.idx`, atomic write via tmp+rename, 5-minute cache TTL.

**Score normalization**: `count(A,B) / max(count(A,*))` — max within A's row, so the most-frequent co-read partner always scores 1.0.

### Server integration

`handleRead` in `internal/app/server/handlers.go` calls `coReadRepo.RecordCoRead(projectRoot, relPath)` as a side effect of every successful file read. `ServerDeps` carries the `CoReadRepo` and `ProjectRoot` fields.

## Consequences

- Git co-change provides durable, cross-session structural signal at the cost of one subprocess pair per `idx related` call.
- The persistent co-read matrix accumulates indefinitely; no pruning policy exists yet (future work).
- The temporal proxy (±2h window) is entirely removed; `findTargetReadTime` and `applyCoRead` are deleted.
- `reason` values now include `"git"` in addition to `"co-read"`, `"term-overlap"`, and `"both"`.
- `applyGitCoChange` silently no-ops on git errors to avoid breaking the command in non-git directories.
