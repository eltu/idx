# version

## Purpose

Show binary version and build date.

## Usage

```bash
idx version
idx --version
idx -v
```

## Arguments

- None.

## Flags

- Global: `--quiet`, `-q`.

## Behavior and Side Effects

- Prints build metadata from compile-time values.
- Read-only command; no filesystem writes.

## Output

- `idx <version> (built <build-date>)`

## Errors

- No command-specific runtime validation errors.

## Examples

```bash
idx version
idx --version
```