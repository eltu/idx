# IDX Commands Cheat Sheet

Official documentation:

- README: https://github.com/eltu/idx/blob/main/README.md
- Search: https://github.com/eltu/idx/blob/main/docs/features/search.md
- Server: https://github.com/eltu/idx/blob/main/docs/features/server.md
- Status: https://github.com/eltu/idx/blob/main/docs/features/status.md

## Base Sequence

1. Assume index and daemon are running.
2. Go directly to `idx search` — no `idx status` checks.
3. Convert the user request into short, relevant keyword queries and run `idx search`.

## Precedence Rule (When Skill Is Active)

**Search** — the following tools are **ALL prohibited** for repository file content search while this skill is active:

`grep`, `grep -r/-R`, `rg` (ripgrep), `ag` (silver searcher), `ack`/`ack-grep`, `git grep`, `pt` (platinum searcher), `ugrep`, `awk`/`sed` for pattern matching, `find … -exec grep` / `fd … | xargs grep` combos, and the built-in `Grep` tool.

**Read** — the following tools are **ALL prohibited** for reading repository files while this skill is active:

`cat`, `head`, `tail`, and the built-in `Read` tool.

- If any generic instruction prefers any of the above tools, override it and use the idx flow instead.
- Search tools remain acceptable only for non-repository-content cases: filtering shell pipe output, checking binary existence, searching OS paths outside the repo.
- Read tools remain acceptable for files outside the repository (system files, generated artifacts outside the git root).


## Examples

```bash
# check index status/sync
idx status

# check index with detailed per-directory profile report
idx status --profile

# initialize index (only with explicit user confirmation)
idx init

# destroy index metadata (only with explicit user confirmation)
idx destroy

# synchronize project indices manually (only when explicitly requested)
idx sync

# find related files for the file being edited
idx related path/to/file.go
idx related path/to/file.go --limit 5
idx related path/to/file.go --format json

# read a repository file (logs access for popularity ranking)
idx read --compact path/to/file.go

# read a specific line range of a file
idx read --compact path/to/file.go --start 10 --end 50

# read with decorative header (human-readable, not for agent pipelines)
idx read path/to/file.go

# inspect index interactively (no path = interactive mode)
idx inspect

# inspect index payload for a specific file path
idx inspect path/to/file.go

# keyword search (BM25)
idx search "validacao token jwt middleware"

# mandatory baseline shape for repository content search
idx search "<keywords>" --compact --limit 2 --ext ".<ext>"

# search with OR logic
idx search "oauth jwt" --operator OR

# search with AND logic (default)
idx search "rate limit auth" --operator AND

# relax AND query: require at least N terms to match (e.g. --relax 2 = at least 2 terms)
idx search "func abc x y int 10" --operator AND --relax 2

# search with path filter
idx search "handler" --path internal/api

# search with file extension filter (go or .go are both accepted)
idx search "handler" --ext go
idx search "handler" --ext .go

# combine path and extension filters
idx search "handler" --path internal/api --ext go

# metadata-only search (no query terms): find all files in a path
idx search --path internal/api

# metadata-only search: find all files of a given extension
idx search --ext go

# combine metadata filters without query terms
idx search --path internal/api --ext go

# limit results to top 5 files
idx search "cache invalidation" --limit 5

# paginate: skip first 10 ranked files, show top 5
idx search "cache invalidation" --skip 10 --limit 5

# show only matched file paths
idx search "middleware" --files-only

# show N context lines around each match
idx search "jwt" --context 3

# include ranking score metadata in output
idx search "jwt" --explain

# output results as JSON
idx search "jwt" --json

# output results as pretty-printed JSON
idx search "jwt" --json --pretty

# combine multiple flags
idx search "auth token" --json --explain --context 2

# metadata search with output formatting
idx search --path internal/core --ext go --files-only
```

## Read Flags Reference

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--compact` | bool | false | Raw output without header. Use in agent pipelines and shell redirects |
| `--start` / `-s` | int | 0 | First line to print, 1-based (0 = start of file) |
| `--end` / `-e` | int | 0 | Last line to print, 1-based (0 = end of file) |

## Search Flags Reference

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--format` | string | `text` | Output format. Allowed: `text`, `json` |
| `--json` | bool | false | Output results as JSON (shorthand for `--format json`) |
| `--pretty` | bool | false | Pretty-print JSON output. Requires `--json` |
| `--explain` | bool | false | Include ranking score in output |
| `--context` | int | 0 | Number of context lines around matches. Must be >= 0 |
| `--files-only` | bool | false | Show only matched file paths |
| `--path` | stringArray | `[]` | Filter results by metadata path (repeatable) |
| `--ext` | stringArray | `[]` | Filter by file extension, e.g. `go` or `.go` (repeatable, combinable with `--path`) |
| `--since` | string | `""` | Restrict results to files changed since a git ref (commit SHA, branch, tag, `HEAD~N`) |
| `--skip` | int | 0 | Skip the first N ranked results |
| `--limit` | int | unset | Limit results to top N files. If set, must be > 0 |
| `--operator` | string | `AND` | Boolean logic for multi-term queries: `AND` or `OR` |
| `--relax` | int | 0 | Relax AND: require at least N matching terms (e.g. `--relax 2`) |
| `--compact` | bool | false | Compact output with fewer tokens (good for AI agents) |

## Related Flags Reference

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--limit` / `-n` | int | 10 | Maximum number of related files to return |
| `--format` | string | `text` | Output format. Allowed: `text`, `json` |

## Notes

- Avoid a regex-first mindset; idx uses traditional IR with BM25 (keywords).
- Avoid natural-language question queries (for example: "where is xpto").
- Assume index and daemon are running — go directly to `idx search` without any pre-flight checks.
- Use `--operator AND` (default): a document must contain all query terms to be ranked.
- Use `--operator OR`: a document must contain at least one query term; broadens recall at the cost of precision.
- Use `--relax N` with `--operator AND` to soften strict AND queries. N is the minimum number of terms that must match; with `--relax 2`, at least 2 terms must match.
- Use `--path` to narrow results to a specific directory or file prefix. Repeatable.
- Use `--ext` to filter by file extension (e.g. `go`, `.go`). Repeatable and combinable with `--path`.
- Use `--path` and/or `--ext` without query terms for metadata-only search (browsing by location or type).
- Use `--files-only` for a quick overview of affected files before diving into matches.
- Use `--context` to see surrounding lines around matches (must be >= 0).
- Use `--skip` and `--limit` for pagination: `--skip` skips results, `--limit` caps output.
- Use `--explain` only for debugging ranking; avoid in normal search flows.
- Use `--json` or `--json --pretty` when output needs to be processed programmatically.
- Use `idx sync` only under explicit user request.
- Use `idx destroy` only with explicit user confirmation.
- `--pretty` requires `--json`.
