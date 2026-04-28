# inspect

## Contract

- Command: `idx inspect <path>`
- Purpose: Read and print index payload for a given path.
- Scope: `<path>/.idx/index.idx`

## Parameters

- Positional args:
- `<path>` (required)

- Flags: none

## Preconditions

- `<path>` must resolve from current working directory.
- Target index file must exist.

## Behavior

- Read-only operation.
- Does not modify any index.

## Output contract

- Prints index content in structured, readable format.

## Examples

```bash
idx inspect .
idx inspect internal/
```

## Common failures

- Missing path argument.
- Invalid path.
- Missing index file under `<path>/.idx/index.idx`.
