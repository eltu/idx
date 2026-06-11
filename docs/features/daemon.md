# daemon

> **Removed in v0.5.0.** The `idx daemon` command (`enable`, `disable`, `status`) has been replaced by `idx agent start`, `idx agent stop`, and `idx agent status`. See [agent.md](agent.md).

The daemon subcommands previously managed background watch processes per project. That responsibility now belongs entirely to the `idx agent` lifecycle commands, which run the file-watch loop inside the same process as the index server.

## Migration

| Old command | New equivalent |
| --- | --- |
| `idx daemon enable .` | `idx agent start` |
| `idx daemon disable .` | `idx agent stop` |
| `idx daemon status` | `idx agent status` |

## Background

- ADR 0020 merged the daemon watch loop and JSON-RPC server into a single self-managing process.
- `idx agent start` spawns the unified process; `idx agent stop` terminates it.
- No separate background watch process is needed or supported.
