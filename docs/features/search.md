# search

## Purpose

Search indexed project content using BM25 ranking and optional metadata filters.

## Usage

```bash
idx search [query terms] [flags]
idx find   [query terms] [flags]   # alias
```

## Arguments

- `query terms` — optional **only** when at least one `--path` or `--ext` is provided.

## Flags

### Output flags

| Flag | Shorthand | Type | Default | Notes |
| --- | --- | --- | --- | --- |
| `--format` | | string | `text`¹ | `text` or `json` |
| `--json` | `-j` | bool | `false` | Shorthand for `--format json` |
| `--pretty` | | bool | `false` | Pretty-print JSON; requires `--json` or `--format json` |
| `--explain` | | bool | `false` | Include BM25 score inline with each file path |
| `--compact` | | bool | `false` | Compact output for AI agents — no ANSI color, `lineNum:content` format, no header or blank separators |
| `--context` | `-c` | int | `0`¹ | Context lines around each match. Must be `>= 0` |
| `--files-only` | `-l` | bool | `false` | Return only file paths |
| `--count` | | bool | `false` | Print only the number of matching files |
| `--time` | | bool | `false` | Show query execution time |

### Filtering flags

| Flag | Shorthand | Type | Default | Notes |
| --- | --- | --- | --- | --- |
| `--path` | `-p` | string array | `[]` | Repeatable metadata-path filter |
| `--ext` | `-e` | string array | `[]` | Repeatable extension filter — accepts `go` or `.go` |
| `--since` | | string | `""` | Restrict results to files changed since a git ref (commit SHA, branch, tag, `HEAD~N`) |

### Ranking flags

| Flag | Shorthand | Type | Default | Notes |
| --- | --- | --- | --- | --- |
| `--operator` | | string | `AND`¹ | `AND` or `OR` |
| `--any` | | bool | `false` | Match any term — shorthand for `--operator OR` |
| `--relax` | | int | (off) | Relax AND: require at least N matching terms. Activates only when query has more terms than N |
| `--relaxation` | | int | `0`¹ | Long-form equivalent of `--relax N` |
| `--popularity-weight` | | float | `0.3`¹ | Boost weight for files frequently read via `idx read`. `0` disables |

### Pagination flags

| Flag | Shorthand | Type | Default | Notes |
| --- | --- | --- | --- | --- |
| `--skip` | | int | `0` | Skip the first N ranked results |
| `--limit` | `-n` | int | `0`¹ | Limit results to top N files; `0` = unlimited |

### Global

| Flag | Shorthand | Notes |
| --- | --- | --- |
| `--quiet` | `-q` | Suppress informational output |

¹ Default sourced from `.idx.yml` (`search.format`, `search.context`, `search.limit`, `search.operator`, `search.relaxation`, `bm25.popularity_weight`) when the file exists. CLI flags always take precedence.

### Deprecated flags (still functional, emit a deprecation warning)

| Deprecated flag | Replacement |
| --- | --- |
| `--agent-compact` | `--compact` |
| `--json-pretty` | `--pretty` |
| `--size` | `--limit` / `-n` |
| `--from` | `--skip` |

## Prerequisites

Requires the background agent to be running. If the agent socket is not reachable, the command fails with `✗ idx agent is not running`. See [agent.md](agent.md) and [errors.md](errors.md).

## Behavior and Side Effects

- Resolves project root and searches all indexed directories.
- Tokenizes and deduplicates query terms.
- Files accessed via `idx read` accumulate a read-count in `.idx/read_log.idx`; this count is used as a ranking boost signal — frequently-read files score higher. The boost uses 14-day exponential decay and is configurable via `bm25.popularity_weight` in `.idx.yml` or `--popularity-weight` (set to `0` to disable).
- Supports metadata-only search when query is empty and at least one `--path` or `--ext` filter is set.
- Applies BM25 + normalization for ranking.
- **`--operator AND`** (default): a document must contain **all** query terms to be ranked.
- **`--any`** (or `--operator OR`): a document must contain **at least one** query term. Proximity bonus is skipped for terms absent from a given document.
- **`--relax N`** (or `--relaxation N`): only active with `AND` operator. When the query has more than N terms, evaluates decreasing term prefixes (removing tokens right to left) down to a single term, ranking by largest matched term count.
- **`--count`**: forces `--files-only` internally and prints only the integer file count to stdout. No headers.
- **`--time`**: appends a muted timing line after results showing the RPC round-trip duration.
- Applies output filters in this order: `files-only` / `count`, then pagination (`skip`, `limit`).
- Uses in-memory cache for ranked results (TTL: configurable via `search.cache_ttl`, default 1 minute).
- Read-only command; no filesystem writes.

## Output

### Text mode (default)

- **Header:** `📁 Found <total> file(s) matching your search`
  - When `--skip` / `--limit` is active: `📁 Found <total> file(s) matching your search (showing <N> with pagination)`
- **File path:** ANSI-colored path, one per result block
  - With `--explain`: `path/to/file.go (score: 0.8432)`
- **Match lines:** tree-prefix format with term highlighting and blank line separator between results:
  ```
  internal/features/search/service.go
    ├── 42: func Run(query string, opts Options) (Results, error) {
    └── 55: }

  ```
- **Stale file:** file in index no longer found on disk:
  ```
  internal/old/removed.go
    └── ⚠ file not found — index is outdated, run idx sync
  ```
- **No results:** `No results found.`

### Compact mode (`--compact`)

Disables ANSI color, header, and tree prefix. Line format: `<lineNum>:<content>`. Designed for AI agent consumption (fewer tokens).

```
internal/features/search/service.go
  42: func Run(query string, opts Options) (Results, error) {
  55: }
```

### JSON mode (`--json` / `--format json`)

- Object: `{"count": N, "results": [...]}`
- With `--files-only`: array of path strings `["a.go", "b.go"]`
- With `--explain`: each result includes `"score"` field
- With `--pretty`: indented JSON

### Other

- `--count`: prints a single integer to stdout (e.g. `7`). No other output.
- `--files-only`: one path per line.
- `--time`: appends `  ⏱  Nms` after the last result line.

## Errors

| Condition | Message |
| --- | --- |
| Missing query (no terms, no `--path`, no `--ext`) | `⚠  Missing search query` with usage hint |
| Unsupported format | `unsupported --format value ... expected one of [text json]` |
| `--pretty` without JSON format | `--pretty requires --format json (or -j)` |
| Negative `--context` | `invalid --context value ... expected a non-negative integer` |
| Negative `--skip` | `invalid --skip value ... expected a non-negative integer` |
| Negative `--from` (deprecated) | `invalid --from value ... expected a non-negative integer` |
| `--size` zero when explicitly set | `invalid --size value ... expected a positive integer` |
| `--limit` zero when explicitly set | `invalid --limit value ... expected a positive integer` |
| Unsupported operator | `unsupported --operator value ... expected one of [AND OR]` |
| `--relax` / `--relaxation` with OR operator | `invalid search.relaxation with --operator "OR": expected "AND"` |
| `--since` with invalid git ref | `invalid git ref "<ref>": <git stderr>` |
| Agent not running | `✗ idx agent is not running` |

## Examples

```bash
# Basic search
idx search "error handling"
idx find "error handling"           # alias

# JSON output
idx search -j "error handling"
idx search -j --pretty "config"

# Filter by extension and path
idx search -e go "func main"
idx search -e go -e ts "func main"
idx search -p internal logger

# OR mode (match any term)
idx search --any "Logger TokenHandler"
idx search --operator OR "Logger TokenHandler"

# AND relaxation (match at least 1 of 3 terms)
idx search --relax 1 "init sync destroy"

# Compact output (AI agents / piping)
idx search --compact "BM25"
idx search --count "TODO"
idx search -l -e md "installation"  # list markdown files only

# Context lines
idx search -c 3 "BM25Tokenizer"

# Pagination
idx search --skip 10 -n 5 "handler"

# Score and timing
idx search --explain "BM25 scoring"
idx search --time --explain "BM25 scoring"

# Metadata-only (no query)
idx search --ext go
idx search --path internal/core

# Git-aware: restrict to files changed since a ref
idx search "error handling" --since HEAD~1
idx search "middleware" --since main --ext go
idx search "config" --since abc1234
```
