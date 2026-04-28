# watch

## Contract

- Command: `idx watch`
- Purpose: Keep indexes synchronized in realtime while process is running.
- Scope: Current Git project.

## Parameters

- Positional args: none

- Flags:
- `--debounce <duration>` (default: `750ms`, must be `> 0`)
- `--show-updated-files` (default: `false`)

## Preconditions

- Must run inside a Git repository.
- Daemon must not already be monitoring the same project.

## Behavior

- Creates root index if missing before entering watch loop.
- Watches directories recursively.
- Skips `.git`, `.idx`, and ignored paths.
- Batches events with debounce and syncs only affected directories.

## Side effects

- Updates index files continuously while running.

## Output contract

- Startup: `👀 Watch mode started. Press Ctrl+C to stop.`
- Batch: `🔄 Synchronized <N> changed directorie(s).`
- Stop: `🛑 Watch mode stopped.`

## Examples

```bash
idx watch
idx watch --debounce 500ms
idx watch --show-updated-files
```

## Common failures

- Invalid debounce value (`<= 0`).
- Watcher initialization errors.
- Permission/read errors in monitored paths.
