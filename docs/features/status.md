# status

## Purpose

Check whether project indexes are current compared to the current filesystem state.

## Usage

```bash
idx status
idx status --profile
```

## Arguments

- None.

## Flags

| Flag | Type | Default | Notes |
| --- | --- | --- | --- |
| `--profile` | bool | `false` | Show a detailed per-directory report and summary table before the overview panel |
| `--quiet`, `-q` | bool | `false` | Suppress informational output |

## Prerequisites

Requires `idx server` to be running. If the server socket is not reachable, the command fails with `✗ idx server not running`. See [server.md](server.md) and [errors.md](errors.md).

## Behavior and Side Effects

- Resolves current directory and project Git root.
- Loads ignore rules from the project root.
- Discovers indexed directories under the project.
- Discovers eligible directories (non-ignored) and keeps only directories that currently contain indexable files.
- Fails if eligible directories with files exist but are not indexed.
- For each indexed directory, checks whether reindexing is needed using current file state and checksum/index snapshot logic.
- Marks a directory as stale when it requires reindexing.
- With `--profile`, prints one panel per indexed directory plus a summary panel before the final overview.
- Read-only command; no writes are performed.

## Output

Default (`idx status`) — styled overview panel:

```
╭──────────────────────────────────────────────────────────╮
│ idx  my-project                                         │
│                                                         │
│ Index    ✅ up to date                                  │
│ Files    847 files · 23 directories · 2.1 MB            │
│ Updated  2 minutes ago  (22:18:30)                      │
│ Daemon   ✅ watching  (PID 4821, since 14:32)           │
│ Config   .idx.yml · 3 overrides active                  │
╰──────────────────────────────────────────────────────────╯
```

Panel rows:

| Row | Content |
| --- | --- |
| `Index` | `✅ up to date` when all indexes are current; `❌ N directory/ies stale — run idx sync` otherwise |
| `Files` | Total indexed files · indexed directories · combined index size on disk |
| `Updated` | Human-readable age of the most recently modified indexed file (e.g. `just now`, `2 minutes ago`, `3 hours ago`, `Jan 2, 2024`) plus the local timestamp in parentheses |
| `Daemon` | `✅ watching (PID N, since HH:MM)` / `⏸ disabled` / `— not configured` |
| `Config` | `<filename> · N overrides active` — only shown when `.idx.yml` exists at the project root |

With `--profile` — per-directory tables appear first, then the overview panel:

```
╭──────────────────────────────────────────────────────────────────────────────────────╮
│ 📂 internal/core/services                                                           │
│ ✅ updated                                                                           │
│                                                                                      │
│ | file                                             | updated | modified_at          | │
│ |--------------------------------------------------|---------|----------------------| │
│ | search_command_service.go                        | ✓       | 2024-01-15T10:30:00Z | │
│ ...                                                                                  │
╰──────────────────────────────────────────────────────────────────────────────────────╯

╭────────────────────────────────────────────────────────╮
│ 📊 Summary                                            │
│ | metric                   | value                    | │
│ |--------------------------|--------------------------|  │
│ | directories checked      | 23                       | │
│ | directories updated      | 23                       | │
│ | files checked            | 847                      | │
│ | files updated            | 847                      | │
│ | latest file modification | 2024-01-15T10:30:00Z     | │
╰────────────────────────────────────────────────────────╯
```

Followed by the overview panel (same as default mode, without a Config row).

## Errors

- No index found under project root:
  - Error: `no index found under project root "<root>": run idx init first`
- Unindexed eligible directories found:
  - Shows warning panel listing the missing directories with action `run idx sync`
  - Error: `unindexed directories found`
- Stale index detected:
  - Shows overview panel with `❌ N directory/ies stale — run idx sync` in the Index row
  - Error: `stale index`
- Current directory/Git root resolution failures.
- Ignore matcher loading failures.
- Directory listing or file-state read failures while checking status.

## Examples

```bash
idx status
idx status --profile
```
