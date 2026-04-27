# ADR 0005: Add Realtime Watch-Based Index Synchronization

## Status

Accepted

## Context

`idx sync` currently requires manual execution after filesystem changes.
This creates friction for users that edit files continuously and expect
search results to include new/updated files immediately.

The solution must:

- Work with minimal local setup.
- Reuse existing indexing and ignore logic.
- Avoid full project reindex for every single file event.

## Decision

Add a new `idx watch` command that monitors project filesystem events and
triggers directory-level incremental reindexing in near real time.

Implementation notes:

- Uses `fsnotify` for cross-platform filesystem notifications.
- Runs once from the project and watches directories recursively.
- Applies a debounce window to batch bursts of events.
- Exposes configurable debounce via `--debounce` (default `750ms`).
- Reuses existing sync/indexing logic (`syncDirectoryIndex`) per affected directory.
- Supports optional `--show-updated-files` output to print deduplicated file updates per batch.
- Respects `.gitignore` and ignores `.git` and `.idx` internals.
- If root index does not exist, creates initial index before watch loop starts.

## Alternatives Considered

### Periodic polling (`sync` on interval)

- Pros: no additional dependency.
- Cons: wasted work, slower reaction, more CPU/IO.

### One watcher process per directory managed manually

- Pros: simple conceptual model.
- Cons: poor UX, higher operational complexity.

## Consequences

### Positive

- Near realtime search freshness without manual `sync`.
- Single command UX (`idx watch`) with low configuration.
- Reuses existing indexing behavior and checksum-based optimization.

### Negative

- Long-running process lifecycle must be managed by user shell/session.
- File watcher backends can emit noisy event bursts; debounce handling is required.
- Additional dependency (`fsnotify`) is introduced.
