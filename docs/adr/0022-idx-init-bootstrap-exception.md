# ADR-0022: idx init executes in-process as a bootstrap exception

## Status

Accepted

## Context

ADR-0019 established that all index-related CLI commands delegate to the server via
JSON-RPC, with "no in-process fallback — missing server is a clear error."

This created a bootstrap paradox for new projects:

- `idx init` requires the server to be running (to forward the RPC call).
- `idx server start` required `.idx/index.idx` to exist (guard added to prevent
  running on uninitialized projects).
- Neither command could succeed first on a clean project.

Additionally, the daemon's watch loop already contained `ensureRootIndex()` which
auto-creates the index when the server starts without one — making the `requireIndexExists`
guard in `ServerDaemonService.Start()` redundant.

## Decision

Two targeted changes resolve the paradox:

1. **`idx init` executes in-process**, not via RPC. In `cmd/idx/main.go`, `isInitCommand`
   routes `idx init` through `runServer()` (the same in-process path used by the daemon
   itself). This lets `idx init` run before a server exists. When the server is already
   running and the user re-runs `idx init`, the index is rebuilt on disk and the server
   picks up changes on the next `idx sync` call.

2. **`idx server start` no longer requires prior init.** The `requireIndexExists` guard
   is removed from `ServerDaemonService.Start()`. The daemon's `ensureRootIndex()` in
   `watch_service.go` already handles the "no index" case gracefully, printing a user-
   friendly message and building the initial index before entering the watch loop.

## Consequences

### Positive

- Both `idx init && idx server start` and `idx server start` alone work on clean projects.
- The bootstrap paradox is eliminated without adding a new command or a complex retry loop.
- No new code paths for the init service itself — `runServer()` already wired it correctly.

### Trade-off

When the server is running and the user executes `idx init` (force re-index), the server's
in-memory state is not updated immediately. The user must run `idx sync` afterwards to
propagate the new index to the running server. This trade-off is acceptable because
forced re-init with a running server is rare, and `idx sync` already covers incremental
updates.
