# inspect

## Purpose

Inspect index payloads either as JSON (single path) or in interactive TUI mode (all project indexes).

## Usage

```bash
idx inspect [path]
```

## Arguments

- `path` (optional): relative or absolute path from the current directory.
- At most one path is accepted.

## Flags

- None.

## Behavior and Side Effects

- Read-only command; does not modify indexes.
- If `path` is provided:
	- Resolves path against current working directory.
	- Validates `<path>/.idx/index.idx` exists.
	- Loads and prints JSON payload of that index.
- If `path` is omitted:
	- Resolves Git project root.
	- Loads all indexed directories under the project.
	- Opens interactive inspect TUI with merged data.

## Output

- With `path`: pretty JSON payload printed to stdout.
- Without `path`: interactive inspect TUI starts.

## Errors

- More than one argument: `inspect accepts at most one path...`
- Invalid path token (empty or starts with `--`): `invalid inspect path ...`
- No index found at target path.
- No index found under project root when running without a path.

## Examples

```bash
idx inspect
idx inspect .
idx inspect internal/
```
