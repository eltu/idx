# inspect

## Purpose

Browse the BM25 index interactively (TUI) or dump a raw index payload as JSON for debugging ranking issues.

## Usage

```bash
idx inspect [path]
```

## Arguments

- `path` (optional): relative or absolute path from the current directory.
- At most one path is accepted.
- Empty values and flag-like tokens (starting with `--`) are rejected.

## Flags

- Global: `--quiet`, `-q`.

## Behavior and Side Effects

- Read-only command; does not modify indexes.
- **Without `path`**: opens the interactive TUI to browse indexed directories, documents, and transaction logs.
  - Navigate with `↑`/`↓` (or `k`/`j`), `PgUp`/`PgDn`.
  - Filter with `/` prefix.
  - Drill down with `Enter`.
  - Quit with `q` or `Ctrl+C`.
- **With `path`**: loads and prints the raw BM25 index for that directory as JSON — useful for debugging ranking issues.

## Output

- With `path`: pretty JSON payload of the index printed to stdout.
- Without `path`: interactive TUI starts in the terminal.

## Errors

- More than one argument: `inspect accepts at most one path...`
- Invalid path token (empty or starts with `--`): `invalid inspect path ...`
- No index found at target path.
- No index found under project root when running without a path.

## Examples

```bash
# Open interactive index browser
idx inspect

# Dump JSON index for a specific directory
idx inspect internal/features/search
idx inspect .
```
