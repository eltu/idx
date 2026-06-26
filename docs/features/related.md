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
| `--format` | | string | `text` | `text` or `json` |

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

## Output

### Text mode (default)

```
  internal/features/search/query_executor.go   (both)          0.92
  internal/features/search/scoring.go          (term-overlap)  0.71
  internal/features/read/service.go            (co-read)       0.54
```

Columns: relative path · signal reason · final score (0–1, two decimals).

### JSON mode (`--format json`)

```json
[
  {"path": "internal/features/search/query_executor.go", "score": 0.92, "reason": "both"},
  {"path": "internal/features/search/scoring.go",        "score": 0.71, "reason": "term-overlap"}
]
```

## Examples

```bash
# Default: up to 10 related files in text format
idx related internal/features/search/service.go

# Limit results
idx related internal/features/search/service.go --limit 5

# JSON output (for scripting or AI agents)
idx related internal/features/search/service.go --format json
```

## See Also

- [search.md](search.md) — full-text BM25 search with `--since` Git filter
- [read.md](read.md) — file streaming (feeds the co-read signal)
- [ADR 0024](../adr/0024-related-command-co-read-term-overlap.md) — design decisions for this feature
