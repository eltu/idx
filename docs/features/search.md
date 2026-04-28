# search

## Contract

- Command: `idx search [query terms]`
- Purpose: Query indexed content across project indexes.
- Scope: All discovered `.idx/index.idx` files in project.

## Parameters

- Positional args:
- `query terms` (optional only when `--path` is provided)

- Flags:
- `--format text|json` (default: `text`)
- `--json-pretty` (requires `--format json`)
- `--context <N>` (`N >= 0`)
- `--matches-only`
- `--files-only`
- `--path <pattern>` (repeatable)
- `--from <N>` (`N >= 0`)
- `--size <N>` (`N > 0` when set)

## Preconditions

- Project indexes must exist (`idx init` first).

## Behavior

- Ranks results using BM25 and normalization.
- Supports metadata-only search when query is empty and `--path` is provided.
- Uses cached ranked results for pagination efficiency.

## Output contract

- `text` format: human-readable list/tree output.
- `json` format: structured result payload.

## Examples

```bash
idx search auth token
idx search auth token --format json --json-pretty
idx search auth token --context 2
idx search --path internal/core
idx search auth token --from 10 --size 5
```

## Common failures

- `missing search query` when no terms and no `--path`.
- Invalid flag values (`--context`, `--from`, `--size`).
- `--json-pretty` without `--format json`.
