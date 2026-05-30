# server

## Purpose

Manage the idx JSON-RPC server daemon. The server holds the BM25 index in memory and serves all index-related CLI commands (`init`, `sync`, `status`, `search`, `read`, `inspect`, `destroy`) over a Unix socket. It also runs the file-watch loop internally — no separate `idx watch` process is needed when the server is running.

## Usage

```bash
idx server <subcommand>
```

## Subcommands

| Subcommand | Description |
| --- | --- |
| `start` | Start the idx server daemon in the background |
| `stop` | Stop the running idx server daemon |
| `status` | Show the current idx server daemon status |

> `idx server run` is a hidden internal command spawned by `start`. Do not call it directly.

## Arguments

- All subcommands: none beyond the subcommand name itself.

## Flags

- Global: `--quiet`, `-q`.

## Behavior and Side Effects

### `idx server start`

- Resolves the Git project root from the current directory by walking up until a `.idx` directory is found.
- Fails immediately with a styled error when not inside an initialized idx project.
- Spawns `idx server run` as a detached background process.
- `idx server run` binds a Unix domain socket at `~/.idx/<sanitized-project-name>.sock`.
- Registers JSON-RPC 2.0 handlers for:
  - `idx.init` → runs `idx init`, returns `{success, output}`
  - `idx.sync` → runs `idx sync`, returns `{success, output}`
  - `idx.status` → runs `idx status`, returns `{success, output}`
  - `idx.inspect` → returns the merged InvertedIndex
  - `idx.search` → executes BM25 search, returns structured results
  - `idx.read` → streams file lines, returns `{lines: [...]}`
  - `idx.destroy` → removes index metadata, returns `{success, output}`
- Runs the file-watch loop concurrently inside the same process. Watch errors are logged but do not stop the server.
- Handles `SIGINT` and `SIGTERM` for graceful shutdown (closes socket, waits for in-flight requests).
- The socket file is removed on clean shutdown. Stale sockets are overwritten on the next start.
- Writes structured JSON logs to `~/.idx/logs/<project-name>/idx.log` (level: `log.level` in `.idx.yml` or `IDX_LOG_LEVEL`).

### `idx server stop`

- Resolves the Git project root.
- Sends `SIGTERM` to the background server process.
- Removes the PID record from daemon state.

### `idx server status`

- Resolves the Git project root.
- Prints the current server process status (running / stopped), PID, and uptime.

## Socket path

```
~/.idx/<sanitized-project-name>.sock
```

`<sanitized-project-name>` is the basename of the Git project root with non-alphanumeric characters replaced by underscores and leading/trailing `._-` stripped.

## Output

- `start`: no output on success; styled error on failure.
- `stop`: no output on success; error if server is not running.
- `status`: formatted status panel with process information.
- `run` (internal): no stdout. Log output goes to the rotating log file only.

## Errors

- Not inside an idx project (no `.idx` directory found walking up from current directory):
  ```
  ✗ Not inside an idx project
    cd <project-root>
    then: idx server start
  ```
- Project not initialized (`.idx` not present):
  ```
  ✗ Project not initialized
    idx init
    then: idx server start
  ```
- Socket path cannot be created (home directory unavailable, permissions).
- Per-request handler errors are returned as JSON-RPC error responses, not as server crashes.

## Relationship to other commands

When `idx server` is **not running**, every command that routes through the socket (`search`, `init`, `sync`, `status`, `read`, `inspect`, `destroy`) fails immediately with:

```
✗ idx server not running
  start with: idx server start
```

Commands that remain in-process and do NOT require the server: `watch`, `skills`, `config`, `version`.

## Examples

```bash
# Start the server daemon in the background
idx server start

# Check server status
idx server status

# Stop the server daemon
idx server stop

# Start fresh: init index then start server
idx init
idx server start

# Verify server is reachable
idx status

# Search via the running server
idx search "BM25" --size 5 --agent-compact
```
