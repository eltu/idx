# ADR 0026: `idx destroy` Stops the Watch Loop Before Removing Indices

## Status

Accepted. Supersedes the implementation described in ADR 0011, which no
longer matches the codebase after the ADR 0019/0020 migration to the
client-server architecture.

## Context

ADR 0011 established that `idx destroy` must disable the daemon before
removing `.idx` directories, to prevent a running watcher from reacting to
the deletions by resyncing (and thereby recreating) the very directories
destroy just removed. That fix was implemented against the pre-ADR-0019
architecture: a separate `idx daemon enable/disable` command pair and a
`DaemonService` that tracked watcher processes in a shared state file.

ADR 0019 and ADR 0020 replaced that architecture with a single
self-managing `idx server` process that embeds both the JSON-RPC listener
and the file-watch loop, reachable only over a Unix socket at
`<project>/.idx/server.sock`. `idx daemon enable/disable` no longer exist.
Destroy now reaches the server via `idx.destroy` over that socket, exactly
like every other index command.

This migration silently dropped the guarantee ADR 0011 had put in place. The
regression had two independent parts, both invisible until a real daemon was
left running during a real destroy:

1. **Orphaned process.** `handleDestroy` removed the project's `.idx`
   directory — which holds `server.sock` and `server.state` — and returned.
   The CLI's separate `stopServerForDestroy()` step then tried to detect and
   `SIGTERM` the agent, but dialed a socket path that destroy itself had just
   deleted. It reported "Agent is not running" and never signaled the
   process, which stayed alive: unreachable, but still running.

2. **Watch loop resurrection.** While the agent was practically unreachable
   from the outside, its embedded watch loop kept running (nothing had
   canceled its context). Removing a directory's `.idx` produces an fsnotify
   `Remove` event. `eventDirectory()` resolves the event's target directory
   via `os.Stat`; since the removed path no longer exists, `os.Stat` fails
   and the code falls back to `filepath.Dir(path)` — the *parent* directory.
   That parent isn't named `.idx`, so `shouldSkipSystemDirectory` does not
   filter the event out, and the watcher schedules a resync of the parent —
   recreating the `.idx` that destroy just deleted. This happened for every
   directory in the project, including the root, because the whole recursive
   delete ran while the watch loop was still fully active.

Both bugs compounded: even a correctly-ordered external stop would not have
helped, since the watch loop's resurrection happens synchronously, inside
the same server process, while it is still handling the destroy RPC.

## Decision

Fix both parts inside the server itself, since destroy's RPC only reaches a
live server and an external client can no longer signal it once `.idx` is
gone.

### 1. Stop the watch loop before removing anything

`newServerRunCommand` (`internal/app/cli/server_command.go`) derives a
child context for the watch goroutine and registers a stop callback on the
server via an optional `SetWatchStopper(stop func())` interface:

```go
watchCtx, cancelWatch := context.WithCancel(ctx)
defer cancelWatch()
watchDone := make(chan struct{})
go func() {
    defer close(watchDone)
    _ = runner.indexCommand.WatchWithContext(watchCtx, defaultServerWatchDebounce)
}()

if setter, ok := runner.indexServer.(watchStopSetter); ok {
    setter.SetWatchStopper(func() {
        cancelWatch()
        <-watchDone
    })
}
```

`handleDestroy` (`internal/app/server/handlers.go`) calls this stopper —
and waits for the watch goroutine to fully exit — before running
`DestroyCommandService.Run()`. No fsnotify events are processed while
`.idx` directories are being removed, so nothing gets resurrected.

### 2. Self-shutdown after a successful destroy

The server stores its own `net.Listener` and closes it as the last step of
a successful `handleDestroy`, via a nil-safe `shutdownAfterDestroy()`
method. Closing the listener does not affect the in-flight connection
still writing the destroy response — that connection was already accepted.
`Serve`'s accept loop then errors on the closed listener, `wg.Wait()`
drains the in-flight handler, and `Serve` returns, exiting the process on
its own.

The CLI's external `stopServerForDestroy()` step is unchanged and kept as a
best-effort fallback; it is now a harmless no-op in the common case, since
the server has usually already exited by the time it runs.

## Decision Drivers

- **Correctness over speed**: the same principle ADR 0011 established —
  destroy must leave zero `.idx` directories on success, even if that costs
  a short wait for the watch loop to stop.
- **No dependency on files destroy is about to delete**: any mechanism for
  stopping the agent that relies on `server.sock` / `server.state` is
  inherently racy against destroy's own deletions. Self-shutdown from
  inside the process sidesteps that entirely.
- **Minimal new surface**: reuse the existing `ServerRunner` embedding
  point (`newServerRunCommand`) rather than introducing a new daemon
  management layer, which is exactly what regressed last time.

## Consequences

### Positive

- `idx destroy` reliably leaves zero `.idx` directories when a server was
  running, including nested per-directory indexes and indexes under hidden
  directories (`.github/`, `.claude/`, etc.).
- The daemon process for the destroyed project actually exits — no more
  accumulating orphaned `idx server run` processes across destroy cycles.
- The fix lives entirely in the client-server architecture's existing
  seams (`ServerDeps`, `ServerRunner`, `newServerRunCommand`); no revival of
  the removed daemon-enable/disable command pair.

### Negative

- `idx destroy` now blocks briefly on `<-watchDone` before removing files,
  since the watch goroutine must fully exit first. In practice this is a
  single select-loop iteration, not a perceptible delay.
- The guarantee depends on `newServerRunCommand` wiring `SetWatchStopper`
  correctly; `handleDestroy` degrades gracefully (skips the stop) if it
  isn't set, which keeps unit tests simple but means a future refactor
  that drops the wiring would silently reintroduce this regression a third
  time. There is no compile-time check tying the two together.

## Operational Notes

- `indexServer.SetWatchStopper` / `shutdownAfterDestroy` live in
  `internal/app/server/server.go`; the call site is `handleDestroy` in
  `internal/app/server/handlers.go`.
- The watch-loop-first ordering is tested in
  `TestHandleDestroy_StopsWatchBeforeRemovingIndexes`
  (`internal/app/server/handlers_test.go`).
- The self-shutdown behavior is tested end-to-end over a real Unix socket in
  `TestServe_HandleDestroy_ShutsDownServerAfterSuccess`.
- Any existing `idx server run` process spawned before this fix predates
  both corrections and will not self-heal; it must be killed manually
  (`pkill -f "idx server run"`) since it is running the old in-memory code.
