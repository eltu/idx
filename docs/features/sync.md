# sync

## Purpose

Synchronize existing project indexes with the current filesystem state.

## Usage

```bash
idx sync
```

## Arguments

- None.

## Flags

- None.

## Behavior and Side Effects

- Must be executed from the Git project root.
- Requires the root index at `<project-root>/.idx/index.idx`.
- Discovers all currently indexed directories.
- Discovers all currently eligible directories based on `.gitignore`.
- Removes stale `.idx` directories that are indexed but no longer eligible.
- Re-indexes eligible directories only when checksums indicate changes.
- Appends changed file entries to `.idx/logs/tlog.idx`.

## Output

- Success: `✅ Project indices synchronized.`

## Errors

- Current directory is not project root.
- Root index does not exist: `sync requires project root to be indexed... run idx init first`.
- `.gitignore` matcher cannot be loaded.
- Directory traversal/read errors.
- Index or checksum persistence errors.

## Examples

```bash
idx sync
```
