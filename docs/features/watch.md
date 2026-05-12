# watch

## Purpose

Run continuous real-time synchronization while the process is active.

## Usage

```bash
idx watch [flags]
```

## Arguments

- None.

## Flags

| Flag | Type | Default | Notes |
| --- | --- | --- | --- |
| `--debounce` | duration | `750ms` | Must be greater than `0` |
| `--show-updated-files` | bool | `false` | Prints changed file list per batch |
| `--quiet`, `-q` | bool | `false` | Suppress informational output |

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

## Output

- Optional startup pre-index messages:
	- `ℹ️ Root index not found. Creating initial index before watch.`
	- `✅ Initial index created. Starting realtime monitoring.`
- Watch start: `👀 Watch mode started. Press Ctrl+C to stop.`
- Batch sync: `🔄 Synchronized <N> changed directorie(s).`
- With `--show-updated-files`:
	- `   updated files:` plus file list, or
	- `   files: none`
- Stop (Ctrl+C / SIGTERM): `🛑 Watch mode stopped.`

## Examples

```bash
idx watch
idx watch --debounce 500ms
idx watch --show-updated-files
```

## Errors

- Invalid debounce value:
	- CLI validation: `invalid --debounce value ... expected a duration greater than 0`
	- Service validation: `failed to run watch command: got invalid debounce ...`
- If daemon state already contains monitored projects:
	- `cannot run watch: daemon is already monitoring this project...`
- Existing daemon watch is allowed when this process is started by daemon (`IDX_DAEMON_CHILD=1`).
- Watcher initialization or runtime watcher errors.
- Directory read/sync/indexing errors during watch batches.
