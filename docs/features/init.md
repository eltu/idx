# init

## Contract

- Command: `idx init`
- Purpose: Create `.idx/index.idx` recursively from the current directory.
- Scope: Current Git project subtree.

## Parameters

- Positional args: none
- Flags: none

## Preconditions

- Must run inside a Git repository.
- Working directory must be readable.

## Behavior

- Skips `.git` and `.idx` directories.
- Applies `.gitignore` rules.
- If root index already exists, it does not rebuild root index.

## Side effects

- Writes index files under `.idx/` for indexed directories.

## Success output

- Typical: `✅ Index created. You can now run idx search.`
- Already indexed: `ℹ️ This project is already indexed. You can run idx search.`

## Common failures

- Not inside Git project root resolution path.
- Permission/read errors while traversing directories.
