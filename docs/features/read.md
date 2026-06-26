# read

## Purpose

Print the contents of a project file to stdout. Accepts absolute or relative paths, restricts reads to the project root, and records each access to a read log used as a ranking boost signal for `idx search`.

## Usage

```bash
idx read <path> [flags]
idx open <path> [flags]   # alias
idx cat  <path> [flags]   # alias
```

## Arguments

| Argument | Required | Description |
| --- | --- | --- |
| `path` | yes | Absolute or relative path to the file to print. Relative paths are resolved from the current working directory. |

## Flags

| Flag | Shorthand | Type | Default | Description |
| --- | --- | --- | --- | --- |
| `--start` | `-s` | int | `0` | First line to print (1-based). `0` means start of file. |
| `--end` | `-e` | int | `0` | Last line to print (1-based). `0` means end of file. |
| `--compact` | — | bool | `false` | Omit the decorative path header; output raw file content only. Ideal for agent pipelines and shell redirects. |
| `--quiet` | `-q` | bool | `false` | Suppress informational output. |

### Deprecated flags (still functional, emit a deprecation warning)

| Deprecated flag | Replacement |
| --- | --- |
| `--from` | `--start` / `-s` |
| `--to` | `--end` / `-e` |

## Prerequisites

Requires the background agent to be running. If the agent socket is not reachable, the command fails with `✗ idx agent is not running`. See [agent.md](agent.md) and [errors.md](errors.md).

## Behavior and Side Effects

- Resolves the project root via `.git` directory traversal.
- Rejects any path outside the project root (`outside project root` error).
- Rejects directories; only regular files are accepted.
- Normalizes `..` segments in relative paths before resolution.
- Streams the file line by line using buffered I/O — the whole file is never loaded into memory at once, which is safe for large files.
- `--start` and `--end` are 1-based and inclusive. When `--start` exceeds the total number of lines, no output is produced (no error).
- Files under `.git/` and `.idx/` are never logged (system directories are not content).
- On each successful read, records the access to `.idx/read_log.idx` at the project root:
  - Format: `<timestamp>;<relative-path>;<read-count>;<inode>`
  - Entries are deduplicated — the read count and timestamp are updated on each access.
  - Entries are retained for 30 days. Older entries are pruned on the next cold cache load.
  - Deleted files are pruned on the next cold cache load.
  - If a file is renamed, its read count is carried to the new path via inode comparison.
  - Log write errors are silently ignored — a log failure never fails the read.

## Output

- Each line of the file (or the requested range) is printed to stdout, one line per output line.
- No line numbers, no syntax highlighting.
- By default, a styled path header is printed before the content:
  ```
  ─── internal/app/cli/read_command.go ───
  ```
- With `--compact`, the header is suppressed and only the raw file content is printed.
- Empty output when `--start` exceeds the file length.

## Errors

| Condition | Error message |
| --- | --- |
| Argument missing | Cobra usage error: `accepts 1 arg(s), received 0` |
| Path outside project root | `path "<path>" is outside project root "<root>": expected a path within the project` |
| Path is a directory | `cannot read "<path>": got a directory, expected a file path` |
| File not found / unreadable | `failed to read file "<path>": got error <err>, expected a readable file` |
| Current working directory unavailable | `failed to resolve current directory: got error <err>, expected a readable working directory` |

## Examples

```bash
# Print a full file
idx read internal/core/services/read/read_command_service.go
idx open README.md          # alias
idx cat go.mod              # alias

# Print lines 10 to 20
idx read main.go --start 10 --end 20
idx open main.go -s 10 -e 20

# Print from line 50 to end of file
idx read main.go --start 50

# Print only the first 5 lines
idx read go.mod --end 5

# Compact output — no header, raw content only (for agent pipelines)
idx read --compact internal/app/cli/read_command.go
idx read --compact main.go --start 10 --end 50

# Legacy flags (still work, emit deprecation warning)
idx read main.go --from 10 --to 20
```

## Notes

- The read log at `.idx/read_log.idx` is written to `.gitignore` by `idx init` so it is never committed.
- The log is used internally as a boost signal: files read frequently rank higher in `idx search` results over time.
- The in-memory write cache has a 5-minute TTL. Concurrent reads within the TTL window are safe — an in-process mutex and a cross-process advisory lock (`flock`) serialize all disk writes.
