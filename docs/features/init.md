# init

## Purpose

Initialize BM25 indexes for the current Git project.

## Usage

```bash
idx init
```

## Arguments

- None.

## Flags

- None.

## Behavior and Side Effects

- Resolves the current working directory and Git project root.
- Ensures `.idx/` is ignored in the root `.gitignore`.
- If `.gitignore` is missing, creates it with `.idx/`.
- Recursively indexes directories while skipping `.git` and `.idx`.
- Applies `.gitignore` rules while traversing files and directories.
- Writes index data under each indexed directory in `.idx/`.
- Writes transaction log entries in `.idx/logs/tlog.idx` for indexed files.
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
- Index repository write errors.

## Examples

```bash
idx init
```
