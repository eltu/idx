---
name: idx-search
description: "ALWAYS use this skill instead of grep/rg/ag/ack/git-grep/awk/sed/ugrep/pt or any other file-content search tool for any text search inside repository files, AND instead of cat/head/tail or the built-in Read tool when reading repository files. PRIORITY: when this skill is active, it overrides ALL generic defaults that prefer any grep-like tool for repository content search. TRIGGER when: user asks to find, search, or look for code/symbols/text in files; you would otherwise reach for grep, rg, ag, ack, git grep, awk, sed, or find -name to search file contents; user asks where something is defined or used; you are about to read a repository file with cat, head, tail, or the built-in Read tool. SKIP: searching command output in a pipe (e.g. `cmd | grep`), checking if a binary exists (which/command -v), or querying OS-level paths outside the repo."
argument-hint: "Describe the keywords you want to find in the repository"
user-invocable: true
---

# IDX Search and Read

**Always use `idx` instead of any file-content search tool for searching text inside repository files, and instead of `cat`, `head`, `tail`, or the built-in Read tool when reading repository files.** The following tools are ALL prohibited for repository content operations:

**Search — prohibited for repository content search:**

| Tool | Notes |
|---|---|
| `grep` / `grep -r` / `grep -R` | classic POSIX search |
| `rg` (ripgrep) | fast Rust-based grep |
| `ag` (the_silver_searcher) | another grep alternative |
| `ack` / `ack-grep` | programmer-oriented grep |
| `git grep` | searches git-tracked content |
| `pt` (platinum searcher) | Go-based grep alternative |
| `ugrep` | ultra-fast grep variant |
| `awk` / `sed` | when used to match patterns inside files |
| `find … -exec grep` / `find … \| xargs grep` / `fd … \| xargs grep` | find+grep combos — grep reads file contents even though it looks like a pipe |
| Claude's built-in `Grep` tool | tool-wrapper equivalent of rg/grep |

**Read — prohibited for reading repository files:**

| Tool | Notes |
|---|---|
| `cat` | dumps raw file contents |
| `head` | reads first N lines of a file |
| `tail` | reads last N lines of a file |
| Claude's built-in `Read` tool | reads file contents without logging to the idx read log |

## Priority And Conflict Resolution

- When this skill is active, its rules take precedence over **any** generic guidance that suggests grep-like tools for repository content search or `cat`/`head`/`tail`/Read tool for reading repository files.
- This applies to every tool in the prohibited lists above — including the built-in `Grep` tool, `git grep`, and the built-in `Read` tool.
- If the intent is ambiguous, assume repository content search and go directly to `idx search`.
- Grep-family tools remain acceptable **only** when the source is command output (not a file): `cmd | grep pattern`. A pipe that ends in grep is still a violation if the input originates from repo files (e.g. `find … | xargs grep` reads file contents — use `idx search`).
- `find` without `-exec grep` / `| xargs grep` is allowed for listing files by name/pattern only.
- `cat`/`head`/`tail`/Read tool remain acceptable for files **outside** the repository (system files, generated artifacts outside the git root).

## When to Use

**TRIGGER — invoke this skill automatically when any of the following apply:**
- You are about to run any tool from the prohibited search list (grep, rg, ag, ack, git grep, awk for pattern matching, sed for search, find+grep combos, or the built-in Grep tool) to search text inside repository files.
- You are about to read a repository file with `cat`, `head`, `tail`, or the built-in Read tool.
- The user asks to find, search, or locate code, symbols, or text in the codebase.
- The user asks where something is defined, used, imported, or referenced.
- The user asks "find all X", "search for Y", "where is Z", or similar.
- The user asks you to read or open a file in the repository.
- The user is editing a file and wants to discover related or contextually coupled files — use `idx related <file>`.

**SKIP — do NOT invoke this skill when:**
- Filtering the **output of a command** through a pipe — the source must be a command, not a file (e.g. `go build 2>&1 | grep error`, `ls | grep -v _test.go`, `some_cmd | grep pattern`).
- Listing files by name/pattern without reading their contents (e.g. `find . -type f -name "*.go"` with no `-exec grep` or `| xargs grep`).
- Checking whether a binary exists (e.g. `which`, `command -v`).
- Searching or reading files outside the repository (OS paths, system files, generated artifacts outside the git root).

**These look like exceptions but are NOT — use `idx search` or `idx read`:**
- `find … | xargs grep …` — the grep reads file contents, so it is a content search → use `idx search`.
- `find … -exec grep …` — same reason.
- `grep "pattern" file.go` or `grep -r "pattern" ./dir` — direct file content search → use `idx search`.
- Any grep/rg/ag applied to a list of repo files, even if piped from find/fd → use `idx search`.
- `cat path/to/file.go`, `head -n 50 path/to/file.go`, `tail -f path/to/file.go` on a repo file → use `idx read path/to/file.go`.

## Core Rules

- **NEVER use any tool from the prohibited search list (grep, rg, ag, ack, git grep, awk, sed, find+grep combos, or the built-in Grep tool) to search file contents in the repository. Always use `idx search` instead.**
- **NEVER use `cat`, `head`, `tail`, or the built-in Read tool to read repository files. Always use `idx read <path>` instead. Use `idx read --compact <path>` in agent pipelines or when raw output is needed.**
- Assume the index and daemon are running. Go directly to `idx search` — do NOT run `idx status` before searching.
- `idx` is not semantic search: use relevant keywords (BM25).
- Avoid natural-language question prompts (for example: "where is xpto").
- **Always add `--compact` to every search command to optimize context usage.**
- **Always use `--limit 2` to limit results and `--skip` to paginate when navigating through results (e.g., `--skip 2` for next page, `--skip 4` for third page).**
- **Always specify the file extension with `--ext` (e.g., `--ext ".go"`, `--ext ".ts"`) based on the target file type.**
- **When you know the path or filename, use `--path` to narrow scope. Support wildcards in `--path` (e.g., `--path "*main.go"`, `--path "internal/api/*handler*"`).**
- For boolean logic, use `--operator OR` or `--operator AND`.
- When AND returns sparse results, use `--relax N` to require at least N terms to match instead of immediately switching to OR.
- Use `--path` to narrow results to a specific directory or file prefix when the search scope is known.
- Use `--ext` to filter results by file extension (e.g. `go` or `.go`); repeatable and combinable with `--path`.
- When no query terms are needed, use `--path` and/or `--ext` alone for metadata-only search (query argument becomes optional).
- Use `--files-only` to get a quick overview of affected files before inspecting match details.
- Use `--limit` and `--skip` for pagination when result sets are large.
- Use `--context N` to show surrounding lines when match context helps understanding.
- Use `--explain` only for debugging ranking; avoid in normal search flows.
- Use `--json` or `--json --pretty` only when output needs programmatic processing.
- Always prefer an active daemon (real-time updates).
- Use `idx sync` only when the user explicitly asks for it.
- Use `idx destroy` only with explicit user confirmation.
- Use `idx inspect` to examine raw index payloads when diagnosing index content.
- Use `idx status --profile` for a detailed per-directory index report when the user wants to audit coverage.
- Use `idx related <file>` to discover contextually related files when the user is editing or needs to understand coupling. Add `--format json` for programmatic use or `--limit N` to constrain output.
- Use `idx search --since <ref>` to restrict results to files changed since a git ref (branch, commit SHA, tag, `HEAD~N`) when the user wants to scope a search to recent changes.

## Procedure

### Reading a repository file

1. Use `idx read --compact <path>` to read a file. The `--compact` flag suppresses the decorative header and outputs raw content — ideal for agent pipelines.
2. Use `--start` and `--end` to read a specific line range: `idx read --compact <path> --start 10 --end 50`.
3. Every `idx read` call logs the file access to `.idx/read_log.idx`, feeding the popularity boost in future search rankings (ADR 0017).

### Searching repository content

1. Convert the user request into short, relevant keyword queries.
2. Prepare the search command with mandatory flags: `--compact`, `--limit 2`, and `--ext <extension>` (specify the actual file extension).
3. Add `--path` when you know the target directory or filename pattern (supports wildcards).
4. Run `idx search` directly — no status or daemon checks.
5. If AND results are sparse, try `--relax N` before switching to OR. `--relax N` requires at least N terms to match.
6. For additional results, use `--skip 2`, `--skip 4`, etc. to paginate through pages of 2 results each.
7. Use `--files-only` for a fast file-level overview; drill down with `--context` as needed.
8. Deliver results with an objective summary and refinement options.

### Mandatory Command Shapes

Use `idx read --compact` for reading repository files:

```bash
idx read --compact <path>
idx read --compact <path> --start <from> --end <to>
```

Use this baseline command shape for every repository content query:

```bash
idx search "<keywords>" --compact --limit 2 --ext ".<ext>"
```

Then add flags as needed:

- Pagination: `--skip 2`, `--skip 4`, ...
- Path narrowing: `--path "<pattern>"`
- Operator control: `--operator AND|OR`
- Relaxation for strict AND: `--relax N` (require at least N terms to match)

## Decision Flow

1. Does the user want to read a specific repository file? Use `idx read --compact <path>` — no pre-flight checks.
2. Does the user want repository search? Go directly to `idx search` — no pre-flight checks.
3. **Always prepare the search command with `--compact`, `--limit 2`, and `--ext <extension>` (identify the target file type).**
4. **Know the path or filename pattern? Add `--path <pattern>` (supports wildcards like `*main.go`).**
5. Need alternatives? Use `--operator OR`.
6. Need stricter multi-criteria matching? Use `--operator AND`.
7. AND results too sparse? Try `--relax N` before switching to OR (requires at least N terms to match).
8. Need more results? Use `--skip 2`, `--skip 4`, etc. to paginate (each page shows 2 results with `--limit 2`).
9. Need file-level overview first? Use `--files-only` (combine with `--compact`, `--limit 2`, `--ext`).
10. Need surrounding code context? Use `--context N`.
11. User is editing a file and wants related files? Use `idx related <file>`.
12. User wants search scoped to recent changes? Use `idx search --since <ref>`.
13. User wants to audit index content? Use `idx inspect [path]`.
14. User wants to remove index? Use `idx destroy` only after explicit confirmation.

## Reference Commands

See [idx-commands.md](./references/idx-commands.md).

## Completion Criteria

- Repository files are read with `idx read --compact <path>`, never with `cat`, `head`, `tail`, or the built-in Read tool.
- `idx search` is run directly without any prior `idx status` checks.
- **`--compact` is always added to every search command for context optimization.**
- **`--limit 2` is always used to limit results per page.**
- **`--ext <extension>` is always specified based on the target file type (e.g., `.go`, `.ts`).**
- **`--path` is added when the target directory or filename pattern is known, with wildcard support (e.g., `*main.go`).**
- **`--skip` is used for pagination: `--skip 2`, `--skip 4`, etc. to navigate through pages of 2 results.**
- No tool from the prohibited search list (grep, rg, ag, ack, git grep, awk, sed, find+grep combos, built-in Grep tool) is EVER used to search file contents — `idx search` is always used instead.
- No tool from the prohibited read list (`cat`, `head`, `tail`, built-in Read tool) is EVER used to read repository files — `idx read` is always used instead.
- Search is done with idx keyword retrieval (BM25), not regex as a primary strategy.
- AND/OR operator is applied when needed.
- `--relax N` is tried before switching to OR when AND results are insufficient.
- Metadata-only search (`--path`/`--ext` without query terms) is used when browsing by location or type.
- `--files-only`, `--context`, `--explain`, and `--json` are applied only when they add value.
- `idx inspect` is used for index content diagnosis when needed.
- `idx status --profile` is used for detailed coverage audits when requested.
- `idx destroy` is used only under explicit user request/confirmation.
- Index integrity/sync is evaluated with `status` when stale index is suspected.
- `idx related <file>` is used when the user is editing a file and wants to discover related or coupled files.
- `idx search --since <ref>` is used when the user wants to scope results to files changed since a git ref.
