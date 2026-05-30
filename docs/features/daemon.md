# daemon

> **Removed in v0.5.0.** The `idx daemon` command (`enable`, `disable`, `status`) has been replaced by `idx server start`, `idx server stop`, and `idx server status`. See [server.md](server.md).

The daemon subcommands previously managed background watch processes per project. That responsibility now belongs entirely to the `idx server` lifecycle commands, which run the file-watch loop inside the same process as the JSON-RPC server.

## Migration

| Old command | New equivalent |
| --- | --- |
| `idx daemon enable .` | `idx server start` |
| `idx daemon disable .` | `idx server stop` |
| `idx daemon status` | `idx server status` |

## Background

- ADR 0020 merged the daemon watch loop and JSON-RPC server into a single self-managing process.
- `idx server start` spawns the unified process; `idx server stop` terminates it.
- No separate background watch process is needed or supported.
