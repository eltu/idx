# server

## Purpose

Start the persistent idx JSON-RPC 2.0 server on a Unix domain socket. All other idx commands (`search`, `init`, `sync`, `status`, `read`) connect to this server instead of running their logic in-process. The server keeps the BM25 index warm in memory across calls.

## Usage

```bash
idx server
```

## Arguments

- None.

## Flags

- Global: `--quiet`, `-q`.

## Behavior and Side Effects

- Resolves the Git project root and derives the socket path:
  `~/.idx/<sanitized-project-name>.sock`
- Binds a Unix domain socket at the computed path.
- Registers JSON-RPC 2.0 handlers for:
  - `idx.init` → runs `idx init` and returns `{success, output}`
  - `idx.sync` → runs `idx sync` and returns `{success, output}`
  - `idx.status` → runs `idx status` and returns `{success, output}`
  - `idx.search` → executes BM25 search and returns structured results
  - `idx.read` → streams file lines and returns `{lines: [...]}`
- Accepts one connection per request; closes the connection after writing the response.
- Handles `SIGINT` and `SIGTERM` for graceful shutdown (closes socket, waits for in-flight requests).
- The socket file is removed on clean shutdown. If the process is killed, the stale socket is overwritten on the next start.
- Writes structured JSON logs to `~/.idx/logs/<project-name>/idx.log` (log level controlled by `log.level` in `.idx.yml` or `IDX_LOG_LEVEL` environment variable).

## Output

- No startup output to stdout. The server is ready when the process is running and the socket file exists at the computed path.
- Log output is written to the rotating log file only.

## Socket path

The socket is placed at:
```
~/.idx/<sanitized-project-name>.sock
```

Where `<sanitized-project-name>` is derived from the basename of the Git project root with non-alphanumeric characters replaced by underscores and leading/trailing `._-` stripped.

## Starting and stopping

Start in the foreground (blocks until interrupted):
```bash
idx server
```

Start in the background (common for scripted use):
```bash
idx server &
```

Stop:
```bash
kill <pid>          # sends SIGTERM, triggers graceful shutdown
# or Ctrl-C when running in foreground
```

Alternatively, use `idx daemon enable .` to have the daemon manage the server lifecycle.

## Errors

- Project root cannot be resolved (not inside a Git repository).
- Socket path cannot be created (home directory unavailable, permissions).
- Socket bind failure (address already in use if a stale socket exists — restart clears it).
- Per-request handler errors are returned as JSON-RPC error responses, not as server crashes.

## Relationship to other commands

When `idx server` is NOT running, every command that routes through the socket (`search`, `init`, `sync`, `status`, `read`) fails immediately with:

```
✗ idx server not running
  start with: idx server
  or:          idx daemon enable .
```

Commands that remain in-process and do NOT require the server: `daemon`, `watch`, `inspect`, `skills`, `config`, `version`, `destroy`.

## Examples

```bash
# Start the server (foreground)
idx server

# Start in background and wait for socket to appear
idx server &
sleep 0.5

# Verify it's reachable
idx status

# Search via the running server
idx search "BM25" --size 5 --agent-compact

# Stop gracefully
kill %1
```
