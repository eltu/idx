# destroy

## Purpose

Remove `.idx` metadata recursively from the current project.

## Usage

```bash
idx destroy
```

## Arguments

- None.

## Flags

- Global: `--quiet`, `-q`.

## Behavior and Side Effects

- Before destroy logic runs, CLI tries to disable daemon monitoring for `.`.
- Daemon disable errors are ignored when they indicate a non-active daemon state.
- Resolves current directory and Git root.
- Must run from the project root.
- Recursively traverses directories.
- Skips `.git` directories.
- Removes every `.idx` directory tree found.

## Output

- Success: `🧹 Index metadata removed from project.`

## Errors

- Current directory cannot be resolved.
- Current directory is not project root.
- Directory traversal read errors.
- One or more `.idx` directories could not be removed.

## Examples

```bash
idx destroy
```
