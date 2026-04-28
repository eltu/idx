# sync

## Contract

- Command: `idx sync`
- Purpose: Synchronize existing indexes with current filesystem state.
- Scope: All directories that already have an index.

## Parameters

- Positional args: none
- Flags: none

## Preconditions

- Must run from the Git project root.
- Root index must exist.

## Behavior

- Rebuilds indexes for already indexed directories.
- Does not create new indexes in non-indexed directories.

## Side effects

- Rewrites existing `.idx/index.idx` files.
- Updates checksum and sync metadata.

## Success output

- `✅ Project indices synchronized.`

## Common failures

- Running outside Git root.
- Missing root index.
- Read/write permission errors.
