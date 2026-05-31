# ADR 0021 — Remove `idx watch` command

## Status

Accepted

## Context

`idx watch` ran a foreground file-watcher that re-indexed changed directories.
ADR 0020 merged the watch loop into the `idx server` process: `idx server start`
spawns a single daemon that runs the JSON-RPC server and the watch loop concurrently.

After ADR 0020, `idx watch` was redundant:
- When the server is running, the command printed "not available in server mode".
- Without the server, re-indexing ran but search/init/sync were unreachable via RPC.

## Decision

Remove `idx watch` from the CLI surface. The internal watch loop (`WatchWithContext`)
is retained in `InitCommandService` and continues to be invoked by `idx server run`.

The `watch.debounce` configuration key is preserved; it is applied to the server's
internal watch loop.

## Consequences

- `idx watch` no longer appears in `idx --help`.
- Users who relied on `idx watch` should use `idx server start` instead.
- The `Watch(showUpdatedFiles, debounce)` method and the daemon-check logic
  (`watchStartedByDaemon`, `currentProjectAlreadyMonitored`) are removed from
  `InitCommandService`.
- `WatchWithContext` remains as the internal API consumed by `newServerRunCommand`.
