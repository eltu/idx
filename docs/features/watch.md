# watch

> **Removed.** `idx watch` was removed in favour of `idx agent start`, which runs the file-watch loop internally as part of the background agent process.

## Migration

| Before | After |
|---|---|
| `idx watch` | `idx agent start` (watch is automatic) |
| `idx watch --debounce 500ms` | Set `watch.debounce: 500ms` in `.idx.yml` |

The `watch.debounce` configuration key is still honoured by the agent's internal watch loop.

## Background

See [ADR 0021](../adr/0021-remove-idx-watch-command.md) and
[ADR 0020](../adr/0020-server-as-self-managing-daemon.md).
