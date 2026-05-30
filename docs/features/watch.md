# watch

## Purpose

Run continuous real-time synchronization while the process is active. When using `idx server`, watching is handled automatically by the server — use `idx watch` only when running without a server daemon.

## Usage

```bash
idx watch [flags]
```

## Arguments

- None.

## Flags

| Flag | Type | Default | Notes |
| --- | --- | --- | --- |
| `--debounce` | duration | `750ms`¹ | Must be greater than `0` |
| `--show-updated-files` | bool | `false` | Prints changed file list per batch |
| `--quiet`, `-q` | bool | `false` | Suppress informational output |

¹ Default is sourced from `.idx.yml` `watch.debounce` when the file exists. CLI flag always takes precedence.

## Behavior and Side Effects

- Resolves current directory and Git root.
- Loads `.gitignore` matcher.
- If root index is missing, creates it before starting watch mode.
- Recursively registers filesystem watches.
- Skips `.git`, `.idx`, and ignored paths.
- Tracks create/write/rename/remove events.
- Batches events by debounce window.
- Re-indexes changed directories.
- When `--show-updated-files` is enabled, prints the relative changed files per batch.
- When `idx server` is running, `idx watch` is not available over RPC — the server manages watching internally.

## Output

- Optional startup messages (when no root index exists):
	- `ℹ  No index found. Creating initial index...`
	- `✓  Initial index created.`
- Watch header: `👀  Watching  <project-name>  ·  Ctrl+C to stop`
- Per-batch sync line: `✦  <timestamp>  <N> dir(s)  <file-summary>` (e.g. `3 file(s)` or `structural change`)
- With `--show-updated-files`, each changed file is printed after the batch line:
	- `   updated files:`
	- `   ├─ <path>` / `   └─ <path>` (last entry uses `└─`)
	- `   └─ ... and <N> more` when more than 5 files changed
	- `   files: none` when no file-level changes detected
- Stop (Ctrl+C): `🛑  Watch stopped.`

## Examples

```bash
idx watch
idx watch --debounce 500ms
idx watch --show-updated-files
```

## Errors

- Invalid debounce value:
	- CLI validation: `invalid --debounce value ... expected a duration greater than 0`
	- Service validation: `failed to run watch command: got invalid debounce ..., expected duration greater than 0`
- Watcher initialization or runtime watcher errors.
- Directory read/sync/indexing errors during watch batches.
