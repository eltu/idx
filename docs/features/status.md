# status

## Purpose

Verify whether indexed files are up to date using the latest transaction log entry in each indexed directory.

## Usage

```bash
idx status
```

## Arguments

- None.

## Flags

- None.

## Behavior and Side Effects

- Resolves the current Git project root.
- Discovers all indexed directories under that project.
- For each indexed directory:
  - Reads `.idx/logs/tlog.idx`.
  - Uses only the latest non-empty log line.
  - Extracts `path`, `hash`, and `indexed_at` fields.
  - Reads the current file modification time for `path`.
  - Compares `indexed_at` and file modification time in UTC (truncated to second precision).
- Read-only command; no writes are performed.

## Output

- Success when every indexed directory passes validation:
  - `✅ Indices are up to date.`

## Errors

- No indexes found under project root.
- Missing `.idx/logs/tlog.idx` in an indexed directory.
- Empty or malformed last transaction-log entry.
- `indexed_at` is not a valid RFC3339 timestamp.
- Referenced file path does not exist.
- Timestamp mismatch between `indexed_at` and actual file modification time (`stale index record`).

## Examples

```bash
idx status
```
