# ADR 0020: Server as Self-Managing Daemon with Embedded Watch

## Status

Accepted

## Context

The project previously had two independent background-process subsystems:

- `idx daemon enable/disable/status` — spawned a separate `idx watch` process per project via a `DaemonService` that maintained a multi-project state file (`~/.idx/daemon.state`).
- `idx server` — ran a JSON-RPC listener in the foreground with no lifecycle management.

This design forced users to manage two distinct processes and remember two separate command groups. It also created a dependency injection cycle: `DaemonService` needed `InitCommandService` (to call init before watching), and `InitCommandService` needed `ProjectMonitorChecker` (implemented by `DaemonService`), requiring a `SetInitCommand` setter to break the cycle at runtime.

## Decision

Collapse both subsystems into a single self-managing daemon per project: **`idx server run`** — a hidden process that runs the JSON-RPC listener and the file-watch loop concurrently under a shared cancellation context.

### New CLI surface

| Command | Behaviour |
|---|---|
| `idx server start` | Spawns `idx server run` in background, saves PID state |
| `idx server stop` | Sends SIGTERM to the process, removes state file |
| `idx server status` | Prints PID, uptime, and socket path |
| `idx server run` | Hidden — the real daemon process (server + watch) |
| `idx watch` | Retained — manual foreground mode for dev/debug |

`idx daemon enable/disable/status` is removed entirely.

### Process routing in `main.go`

`isServerCommand` returns `true` **only for `idx server run`**, routing it to `runServer()`. All other subcommands (`start`, `stop`, `status`) are client-side and route through `runClient()`.

### Concurrency model

`newServerRunCommand` uses `errgroup` to run two goroutines under a `signal.NotifyContext` (SIGINT/SIGTERM):

```
errg.Go(func() { return runner.indexServer.Serve(ctx) })
errg.Go(func() { return runner.indexCommand.WatchWithContext(ctx, debounce) })
```

When either exits (signal received or fatal error), the context is cancelled and both goroutines shut down cleanly.

### DI cycle elimination

`ServerDaemonService.Start` no longer calls `Init`. The embedded watch loop calls `ensureRootIndex` automatically on the first event, making the explicit init call redundant. This removes the bidirectional dependency: only `InitCommandService → ServerDaemonService` (as `ProjectMonitorChecker`) remains.

### State persistence

Per-project state is stored at `~/.idx/<sanitized-project-name>.server.state` (JSON), containing:

```json
{
  "pid": 12345,
  "started_at": "2026-05-27T10:00:00Z",
  "socket": "/home/user/.idx/myproject.sock",
  "project_path": "/home/user/myproject"
}
```

Orphan detection: on `Start`, if a state file exists but the PID is no longer alive, the stale state is removed and a fresh daemon is spawned.

### Watch context propagation

`consumeWatchEvents` gained a `context.Context` parameter with a `case <-ctx.Done(): return nil` branch, ensuring the watch goroutine exits when the server shuts down rather than blocking on the `watcher.Events` channel indefinitely.

## Consequences

**Positive:**
- Single command group for server lifecycle; simpler mental model for users.
- One process per project instead of two, reducing resource overhead.
- DI cycle eliminated; no runtime setter (`SetInitCommand`) needed.
- `idx search` and other client commands work unchanged — they still dial the Unix socket.
- `idx destroy` stops the server daemon before removing indices via `stopServerForDestroy`.

**Negative:**
- `idx watch` no longer starts a persistent background daemon; users who relied on `idx daemon enable .` must migrate to `idx server start`.
- The hidden `idx server run` subcommand must not be called directly by users; it is an implementation detail of the daemon lifecycle.

## Related ADRs

- ADR 0006: Original daemon management decision (superseded by this ADR).
- ADR 0019: IPC via JSON-RPC 2.0 over Unix socket (unchanged; server implementation is the same).
- ADR 0005: Real-time watch mode (unchanged; `WatchWithContext` reuses the existing watch loop).
