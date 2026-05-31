# sync

## Purpose

Synchronize existing project indexes with the current filesystem state. Only directories with modified content are re-indexed — unchanged directories are skipped via checksum comparison.

## Usage

```bash
idx sync
idx update   # alias
```

## Arguments

- None.

## Flags

- Global: `--quiet`, `-q`.

## Prerequisites

Requires `idx server` to be running. If the server socket is not reachable, the command fails with `✗ idx server not running`. See [server.md](server.md) and [errors.md](errors.md).

## Behavior and Side Effects

- Must be executed from the Git project root.
- Requires the root index at `<project-root>/.idx/index.idx`.
- Discovers indexed directories and currently eligible directories from `.gitignore`.
- Removes stale `.idx` directories that are no longer eligible.
- Re-indexes eligible directories only when checksums indicate changes.

## Output

- Success: `✅ Project indices synchronized.`

## Errors

- Current directory is not project root.
- Root index does not exist: `sync requires project root to be indexed... run idx init first`.
- `.gitignore` matcher cannot be loaded.
- Directory traversal/read errors.
- Index/checksum persistence errors.

## Examples

```bash
idx sync
idx update   # alias
```
