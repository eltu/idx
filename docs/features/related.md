# related

## Purpose

Find files most likely related to the file currently being edited, using co-read affinity and BM25 term co-occurrence.

## Usage

```bash
idx related <file> [flags]
```

## Arguments

- `<file>` — path to the target file (relative to cwd or absolute).

## Flags

| Flag | Shorthand | Type | Default | Notes |
| --- | --- | --- | --- | --- |
| `--limit` | `-n` | int | `10` | Maximum number of related files to return |
| `--skip` | | int | `0` | Skip the first N results |
| `--format` | | string | `text` | `text` or `json` |
| `--since` | | string | `""` | Restrict results to files changed since a git ref (commit SHA, branch, tag, `HEAD~N`) |
| `--ext` | `-e` | string array | (none) | Filter results by file extension — accepts `go` or `.go`; repeatable |
| `--compact` | | bool | `false` | Compact output — paths only, no score or reason |

## Signals

Results are ranked by a weighted sum of two signals:

| Signal | Weight | How it works |
| --- | --- | --- |
| Co-read affinity | 0.7 | Files whose `LastReadAt` in the read log falls within ±2 h of the target get a proximity score `1/(1+deltaHours)` |
| Term co-occurrence | 0.3 | BM25Score of the target's term vector against all indexed documents |

The **reason** field in the output indicates which signal(s) contributed:

| Reason | Meaning |
| --- | --- |
| `co-read` | Only co-read affinity was non-zero |
| `term-overlap` | Only term co-occurrence was non-zero |
| `both` | Both signals contributed |

## Fallback when data is insufficient

- If the read log is empty, only term co-occurrence contributes (co-read weight is effectively zero).
- If neither signal produces candidates, the command returns an empty list with the message `No related files found.` — no error.

## Prerequisites

Requires the background agent to be running. See [agent.md](agent.md).

## Behavior and Side Effects

- Filters are applied after ranking. `--since` and `--ext` post-filter the ranked list; `--skip` is applied after filters.
- `--since` invokes `git diff --name-only <ref>...HEAD` relative to the project root. An invalid ref returns an error immediately.
- `--ext` accepts extensions with or without a leading dot (`go` and `.go` are equivalent); repeatable for multiple extensions.
- `--compact` suppresses the score and reason columns and outputs one path per line.
- `--format json` ignores `--compact` — the JSON payload always includes `path`, `score`, and `reason`.

## Output

### Text mode (default)

```
  internal/features/search/query_executor.go   (both)          0.92
  internal/features/search/scoring.go          (term-overlap)  0.71
  internal/features/read/service.go            (co-read)       0.54
```

Columns: relative path · signal reason · final score (0–1, two decimals).

### Compact mode (`--compact`)

```
internal/features/search/query_executor.go
internal/features/search/scoring.go
internal/features/read/service.go
```

One path per line. No score, no reason. Suitable for scripting and AI agent pipelines.

### JSON mode (`--format json`)

```json
[
  {"path": "internal/features/search/query_executor.go", "score": 0.92, "reason": "both"},
  {"path": "internal/features/search/scoring.go",        "score": 0.71, "reason": "term-overlap"}
]
```

## Errors

| Condition | Message |
| --- | --- |
| `<file>` not provided | `Error: accepts 1 arg(s), received 0` |
| `--since` with invalid git ref | `invalid git ref "<ref>": <git stderr>` |
| Agent not running | `✗ idx agent is not running` |

## Examples

```bash
# Default: up to 10 related files in text format
idx related internal/features/search/service.go

# Limit results
idx related internal/features/search/service.go --limit 5

# JSON output (for scripting or AI agents)
idx related internal/features/search/service.go --format json

# Compact output — paths only (AI-friendly)
idx related internal/features/search/service.go --compact

# Restrict to files changed since a git ref
idx related internal/features/search/service.go --since HEAD~1
idx related internal/features/search/service.go --since main

# Filter results by file extension
idx related internal/features/search/service.go --ext go
idx related internal/features/search/service.go --ext go --ext md

# Combine filters
idx related internal/features/search/service.go --since HEAD~3 --ext go --compact

# Paginate results
idx related internal/features/search/service.go --limit 5 --skip 5
```

## See Also

- [search.md](search.md) — full-text BM25 search with `--since` Git filter
- [read.md](read.md) — file streaming (feeds the co-read signal)
- [ADR 0024](../adr/0024-related-command-co-read-term-overlap.md) — design decisions for this feature
