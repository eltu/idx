# agent

## Purpose

Manage the idx background agent. The agent holds the BM25 index in memory, serves all index-related CLI commands (`init`, `sync`, `status`, `search`, `read`, `inspect`, `destroy`) over a Unix socket, and runs the file-watch loop internally — no separate `idx watch` process is needed.

## Usage

```bash
idx agent <subcommand>
```

> **Alias:** `idx server` is retained as a backward-compatible alias.

## Subcommands

| Subcommand | Description |
| --- | --- |
| `start` | Start the background agent |
| `stop` | Stop the running background agent |
| `status` | Show whether the agent is running and healthy |

> `idx agent run` is a hidden internal command spawned by `start`. Do not call it directly.

## Arguments

All subcommands: none beyond the subcommand name itself.

## Flags

### `idx agent status`

| Flag | Shorthand | Type | Default | Description |
| --- | --- | --- | --- | --- |
| `--json` | `-j` | bool | `false` | Output status as JSON instead of formatted text |

### Global

| Flag | Shorthand | Description |
| --- | --- | --- |
| `--quiet` | `-q` | Suppress informational output |

## Behavior and Side Effects

### `idx agent start`

- Resolves the Git project root from the current directory by walking up until a `.idx` directory is found.
- Fails immediately with a styled error when not inside an initialized idx project.
- Spawns `idx server run` as a detached background process (internal command, backward-compatible name retained for spawner reliability).
- The agent process binds a Unix domain socket at `~/.idx/<project-hash>.sock`.
- Registers JSON-RPC 2.0 handlers for all index operations (`idx.init`, `idx.sync`, `idx.status`, `idx.inspect`, `idx.search`, `idx.read`, `idx.destroy`, `idx.config`).
- Runs the file-watch loop concurrently inside the same process. Watch errors are logged but do not stop the agent.
- Handles `SIGINT` and `SIGTERM` for graceful shutdown.
- Writes structured JSON logs to `~/.idx/logs/<project-name>/idx.log`.

### `idx agent stop`

- Resolves the Git project root.
- Sends `SIGTERM` to the background agent process.
- Removes the state record from `.idx/server.state`.

### `idx agent status`

- Resolves the Git project root.
- Default (text): prints a one-line status message.
- With `--json` / `-j`: prints a JSON object to stdout (see below).

## Output

### `idx agent start`

```
✅ Agent started (PID: 12345)
```

If already running:

```
⚡ Agent is already running
```

### `idx agent stop`

```
🛑 Agent stopped
```

If not running:

```
ℹ️  Agent is not running
```

### `idx agent status` (text)

Running with state:

```
✅ Agent running (PID: 12345, uptime: 2m30s)
```

Running without recorded state (edge case — socket alive but state file absent):

```
✅ Agent running
```

Not running:

```
❌ Agent is not running
```

### `idx agent status --json`

Running:

```json
{
  "running": true,
  "pid": 12345,
  "uptime_seconds": 150,
  "socket_path": "/home/user/.idx/myproject.sock"
}
```

Not running:

```json
{
  "running": false,
  "socket_path": "/home/user/.idx/myproject.sock"
}
```

Fields `pid` and `uptime_seconds` are omitted when `running` is `false`.

## Errors

Not inside an idx project (no `.idx` directory found walking up from current directory):

```
✗ Not inside an idx project
  cd <project-root>
  then: idx agent start
```

When any command requiring the agent cannot reach the socket:

```
✗ idx agent is not running
  start with: idx agent start
  then retry your command
```

## Relationship to other commands

When the agent is **not running**, every command that routes through the socket (`search`, `init`, `sync`, `status`, `read`, `inspect`, `destroy`) fails immediately with the error above.

Commands that remain in-process and do **not** require the agent: `skills`, `config`, `version`.

## Examples

```bash
# Start the background agent
idx agent start

# Check status (human-readable — shows PID for manual kill if needed)
idx agent status

# Check status (machine-readable / AI agents)
idx agent status --json
idx agent status -j

# Kill agent manually using the PID from status
kill 12345

# Stop the agent gracefully
idx agent stop

# Start fresh: init index then start agent
idx init
idx agent start

# Verify agent is reachable
idx status

# Search via the running agent
idx search "BM25" -n 5 --compact
```
