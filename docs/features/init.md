# init

## Purpose

Build the full BM25 index for the current Git project. Safe to re-run: subsequent calls sync only changed directories.

## Usage

```bash
idx init
```

## Arguments

- None.

## Flags

- Global: `--quiet`, `-q`.

## Prerequisites

None. `idx init` executes in-process and does not require the background agent to be running. This makes it safe to use as a bootstrap step on a clean project before the agent has ever been started. See [ADR 0022](../adr/0022-idx-init-bootstrap-exception.md).

> **Note:** if the agent is already running when you re-run `idx init` (force re-index), the agent's in-memory state is not updated immediately. Run `idx sync` afterwards to propagate the new index.

## Behavior and Side Effects

- Resolves the current working directory and Git project root.
- Ensures `.idx/` is ignored in the root `.gitignore`.
- If `.gitignore` is missing, creates it with `.idx/`.
- Recursively indexes directories while skipping `.git` and `.idx`.
- Applies `.gitignore` rules while traversing files and directories.
- Writes index data under each indexed directory in `.idx/`.
- Updates checksum snapshots used by `idx sync` and `idx status`.
- If an index already exists in the current directory, does not rebuild and returns an info message.

## Output

- Success on first initialization:
  ```
  ✅ Index created. You can now run idx search.
     💡 Run `idx agent start` to enable search
  ```
- Already initialized in current directory: `ℹ️ This project is already indexed. You can run idx search.`

## Errors

- Current directory cannot be resolved.
- Current directory is not inside a Git project.
- `.gitignore` cannot be read or written.
- Ignore matcher cannot be built from `.gitignore`.
- Directory/file read errors during traversal.
- Index/checksum persistence errors.

## Examples

```bash
# Initialize a fresh project (no agent needed)
idx init

# Recommended first-time setup
idx init
idx agent start

# Or: let the agent auto-initialize on first watch event
idx agent start
```
