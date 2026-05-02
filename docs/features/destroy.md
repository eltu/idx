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

- None.

## Behavior and Side Effects

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
