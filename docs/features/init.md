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

Requires the background agent to be running. If the agent socket is not reachable, the command fails with `✗ idx agent is not running`. See [agent.md](agent.md) and [errors.md](errors.md).

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

- Success on first initialization: `✅ Index created. You can now run idx search.`
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
idx init
```
