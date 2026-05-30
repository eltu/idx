# ADR 0019 — IPC via JSON-RPC 2.0 over Unix Socket

## Status

Accepted

## Context

`idx` is a CLI tool that builds and queries a BM25 full-text index. The initial design rebuilt the index in-process for every command invocation, which meant cold-start latency on every `idx search` call. The need for IPC arose from two requirements:

1. Other processes (agents, editors, scripts) need to query the index without spawning a full Go binary with BM25 initialisation overhead.
2. A persistent server can keep the index warm in memory and apply incremental updates in the background.

A key constraint was that the transport must be local-only (no network exposure), fast for same-machine IPC, and require no external dependencies.

## Decision

Implement a **persistent `idx server` process** that exposes a JSON-RPC 2.0 interface over a **Unix domain socket**.

The socket path is `~/.idx/<sanitized-project-name>.sock`, derived from the git root basename. All CLI commands (`search`, `init`, `sync`, `status`, `read`) connect to this socket; if the server is not running, the client returns a styled error message and exits with code 1 — there is no in-process fallback.

### Transport

Unix domain socket via `net.Listen("unix", path)` / `net.Dial("unix", path)`. One connection per request; the server closes the connection after writing the response. Content-Length framing (`Content-Length: N\r\n\r\n<body>`) allows message boundaries over a byte stream.

### Protocol

Seven RPC methods:

| Method | Cobra command |
|---|---|
| `idx.search` | `idx search` |
| `idx.init` | `idx init` |
| `idx.sync` | `idx sync` |
| `idx.status` | `idx status` |
| `idx.read` | `idx read` |
| `idx.inspect` | `idx inspect` |
| `idx.destroy` | `idx destroy` |

Commands outside RPC scope (`daemon`, `watch`, `skills`, `config`, `version`) remain in-process on the client side.

### Server architecture

The server constructs a fresh service instance per request using a `captureWriter` — an `output.Writer` implementation that collects lines in memory instead of printing them. This lets the server reuse the existing service layer (which writes formatted text to an `output.Writer`) and extract structured response data:

- **search**: server always calls `SearchCommandService` with `Format: "json"`, captures the single JSON line, and returns a structured `SearchResponse`. The client re-formats this for the terminal.
- **read**: server captures each line from `ReadCommandService` and returns `ReadResponse{Lines: []string}`.
- **init/sync/status/destroy**: server captures text output from the respective service and returns `CommandResponse{Success bool, Output string}`.
- **inspect**: server calls `InitCommandService.LoadInspectIndex()` and returns the raw `*InvertedIndex` serialised as JSON. The client deserialises it and renders the TUI locally — the server never needs a terminal.

Service instances are value types — construction is cheap and correct with a per-request `captureWriter`.

### Client architecture

When the first non-flag argument is not `"server"`, `main.go` builds a `remote.SocketClient` (backed by the socket path) and injects RPC adapters into the `CommandRunner`:

- `remote.RemoteSearcher` → implements `cli.Searcher`
- `remote.RemoteReader` → implements `cli.Reader`
- `remote.RemoteIndexCommand` → implements `cli.indexableCommand` (init/sync/status/inspect)
- `remote.RemoteDestroyCommand` → implements `cli.Runner` (destroy)

`watch` remains unsupported over RPC and prints an informational message when invoked; the server watches internally.

### Error on missing server

When `net.Dial` fails (socket absent or refused), the client prints a styled lipgloss message and returns an error:

```
✗ idx server not running
  start with: idx server
  or:          idx daemon enable .
```

### Package layout

```
internal/shared/jsonrpc/         ← codec + dispatcher (Content-Length framing)
internal/shared/ipc/             ← protocol constants, request/response types, SocketPath helper
internal/app/server/             ← server: accept loop, handlers, captureWriter
internal/app/cli/remote/         ← RPC adapters: SocketClient, RemoteSearcher, RemoteReader, RemoteIndexCommand, RemoteDestroyCommand
```

## Consequences

**Positive:**
- Warm index stays in memory across searches — no cold-start latency after the first `idx init`.
- Clean separation: CLI delivery layer depends only on interfaces, not on BM25 or file I/O.
- Unix socket is local-only (no network surface), uses OS-level permissions, requires no TLS.
- Incremental sync can run in the server background without blocking CLI calls.

**Negative:**
- Users must start `idx server` (or `idx daemon enable .`) before CLI commands work.
- One connection per request is simpler to implement but adds a dial overhead (~0.1 ms locally) per call.
- `watch` is not available over RPC — the server watches internally.

## Alternatives Considered

**Hybrid (in-process fallback):** If the socket is absent, fall back to running the service in-process. Rejected because it hides the "server not running" state from the user, complicates testing, and makes the socket optional rather than authoritative.

**gRPC:** Structured contracts and streaming support, but adds a heavy dependency (protobuf toolchain, generated code) for a local IPC channel with five methods. JSON-RPC 2.0 with hand-written types is simpler and sufficient.

**Named pipe (FIFO):** POSIX but less ergonomic than Unix sockets for request/response; no backpressure or connection semantics.
