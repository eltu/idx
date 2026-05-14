# search

## Purpose

Search indexed project content using BM25 ranking and optional metadata filters.

## Usage

```bash
idx search [query terms] [flags]
```

## Arguments

- `query terms` (optional only when at least one `--path` or `--ext` is provided).

## Flags

| Flag | Type | Default | Notes |
| --- | --- | --- | --- |
| `--format` | string | `text`¹ | Allowed: `text`, `json` |
| `--json-pretty` | bool | `false` | Requires `--format json` |
| `--explain` | bool | `false` | Includes ranking score |
| `--agent-compact` | bool | `false` | Compact text output for agents (no header/footer spacing, simplified lines) |
| `--context` | int | `0`¹ | Must be `>= 0` |
| `--matches-only` | bool | `false` | Keeps only matching lines |
| `--files-only` | bool | `false` | Returns only file paths |
| `--path` | string array | `[]` | Repeatable metadata-path filter |
| `--ext` | string array | `[]` | Repeatable file-extension filter (`go` or `.go`) |
| `--from` | int | `0` | Pagination offset, must be `>= 0` |
| `--size` | int | `0`¹ (unlimited) | If set, must be `> 0` |
| `--operator` | string | `AND`¹ | Boolean operator for multi-term queries: `AND` or `OR` |
| `--relaxation` | string | `""`¹ | Only with `--operator AND`. Format `>N`; activates relaxation only when query has more than `N` terms, then removes trailing terms progressively |
| `--quiet`, `-q` | bool | `false` | Suppress informational output |

¹ Default is sourced from `.idx.yml` (`search.format`, `search.context`, `search.size`, `search.operator`, `search.relaxation`) when the file exists. CLI flags always take precedence over `.idx.yml`.

Compatibility alias:

- Hidden flag `--macthes-only` maps to `--matches-only`.

## Behavior and Side Effects

- Resolves project root and searches all indexed directories.
- Tokenizes and deduplicates query terms.
- Supports metadata-only search when query is empty and at least one metadata filter is set (`--path` and/or `--ext`).
- Applies BM25 + normalization for ranking.
- `--operator AND` (default): a document must contain **all** query terms to be ranked.
- `--operator AND` + `--relaxation >N`: activates only when the query has more than `N` terms, then evaluates decreasing term prefixes (removing tokens from right to left) down to a single term, ranking results by largest matched term count first.
- `--operator OR`: a document must contain **at least one** query term; broadens recall at the cost of precision. Proximity bonus is skipped for terms absent from a given document.
- Applies output filters in this order: `files-only` or `matches-only`, then pagination (`from`, `size`).
- `--files-only` has priority over `--matches-only`.
- Uses in-memory cache for ranked results (TTL: 1 minute) and renews TTL on cache hits.
- Read-only command; no filesystem writes.

## Output

- Text mode with results:
	- Header: `📁 Found <total> file(s) matching your search`
	- Or paginated header: `📁 Found <total> file(s) matching your search (showing <displayed> with pagination)`
- With `--agent-compact`, header/footer spacing is omitted and lines are printed as `<line>:<content>`.
- Text mode without results: `No results found.`
- JSON mode without results:
	- `{"count":0,"results":[]}` (pretty when `--json-pretty` is enabled)
- JSON mode with results:
	- Object with `count` and `results`.
	- `score` only appears when `--explain` is true.
- JSON + `--files-only`:
	- Returns an array of file paths only.

## Examples

```bash
idx search auth token
idx search auth token --format json --json-pretty
idx search auth token --explain
idx search auth token --format json --explain
idx search auth token --agent-compact
idx search auth token --context 2
idx search --path internal/core
idx search --ext go
idx search --path internal/core --ext go
idx search auth token --from 10 --size 5
idx search auth token --files-only
idx search auth token --matches-only
idx search auth token --operator OR
idx search auth token --operator AND
idx search func abc x y int 10 --operator AND --relaxation '>2'
```

## Errors

- Missing input contract: `missing search query... expected idx search <terms>`
- Unsupported format: `unsupported --format value ... expected one of [text json]`
- Invalid pretty/json combination: `--json-pretty requires --format json`
- Invalid numeric constraints:
	- `invalid --context value ... expected a non-negative integer`
	- `invalid --from value ... expected a non-negative integer`
	- `invalid --size value ... expected a positive integer`
- Unsupported operator: `unsupported --operator value ... expected one of [AND OR]`
- Invalid relaxation format/operator combination:
	- `invalid --relaxation value ... expected format >N where N is a non-negative integer`
	- `invalid --relaxation with --operator ... expected AND`
- Index/file access errors when loading indexes or source files.
