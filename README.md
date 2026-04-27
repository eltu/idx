# idx

BM25-based code and text search CLI with per-directory indexes.

The goal of `idx` is to provide fast and relevant search for Git projects while keeping indexing costs low by storing one `.idx/index.idx` per directory.

## Core ideas

- Recursive indexing per directory (one BM25 corpus per folder).
- Global project search by loading every discovered index.
- BM25 ranking with proximity bonus between terms.
- Per-directory score normalization for fairer cross-directory comparison.
- Tree-like output showing all matching lines with whole-word token matching.

## Requirements

- Go `1.26+`
- A Git repository (required to resolve the project root from `.git`).

## Build, test and run

Use Makefile targets for local development and CI parity.

Build binaries:

```bash
make build
```

Generated binaries:

- bin/idx

Run tests:

```bash
make test
```

Run format + lint + tests (without build):

```bash
make check
```

Format Go files:

```bash
make fmt
```

Run linter:

```bash
make lint
```

Run cyclomatic complexity check (fails if any non-test function is above 15):

```bash
make complexity
```

Clean build artifacts:

```bash
make clean
```

Run with Go directly (optional):

```bash
go run cmd/idx/main.go <command>
```

Run compiled binary:

```bash
./bin/idx <command>
```

## Available commands

### 1) `init`

Creates `.idx/index.idx` files starting from the current directory and traversing subdirectories recursively.

- Ignores `.git` and `.idx`.
- Respects `.gitignore` rules.
- If an index already exists in the current directory, it does not reindex and reports that the project is already indexed.

Usage:

```bash
idx init
```

Success output:

```text
✅ Index created. You can now run idx search.
```

Output when an index already exists in the current directory:

```text
ℹ️ This project is already indexed. You can run idx search.
```

### 2) `sync`

Resynchronizes all existing project indices.

Rules for the current implementation:

- Must run from the Git project root.
- The project root must already have `.idx/index.idx`.
- Traverses the whole project, finds every existing index, and rebuilds each one using the current documents of that directory.
- Does not create indices for directories that are not already indexed.

Usage:

```bash
idx sync
```

Success output:

```text
✅ Project indices synchronized.
```

### 3) `inspect <path>`

Reads the binary index at the provided path and prints it as pretty JSON.

Usage:

```bash
idx inspect internal/
idx inspect .
```

Rules:

- The provided path is resolved from the current working directory.
- The command expects an index file at `<path>/.idx/index.idx`.
- This command is read-only and never creates or rewrites indexes.

### 4) `search <terms...>`

Searches terms across the whole project (all directories containing `.idx/index.idx`).

Usage:

```bash
idx search tree
idx search module idx
idx search bm25 tokenizer
idx search module idx --format json
idx search module idx --format json --json-pretty
idx search module idx --context 2
idx search module idx --format json --matches-only
idx search module idx --files-only
idx search module idx --format json --files-only
idx search module idx --format json --size 5
idx search module idx --format json --from 10 --size 5
idx search module idx --path internal/core
idx search --path internal/core
```

Current behavior:

- Multi-term query with AND semantics for ranking: documents must contain all terms to score.
- BM25 score with proximity bonus between consecutive terms.
- Per-directory score normalized to `[0, 1]` before global aggregation.
- Line matching is whole-word/token only (no mid-word substring matches).
- Returns all lines in each file that contain at least one query term.
- File paths are shown relative to the project root.
- Optional `--format json` outputs a machine-readable JSON array.
- Optional `--json-pretty` pretty-prints JSON output for humans (requires `--format json`).
- Optional `--context <N>` includes `N` surrounding lines before/after each match.
- Optional `--matches-only` keeps only direct matched lines in the output.
- Optional `--files-only` returns only file paths, ignoring matches and context (deduplicates results by file).
- Optional `--from <N>` skips the first `N` ranked files (zero-based offset).
- Optional `--size <N>` limits output to the top `N` files.
- Optional `--path <pattern>` filters by indexed metadata path (repeatable).
- Metadata-only search is supported with `--path` even when query terms are empty.
- Search results are cached for 1 minute to speed up pagination (`--from` / `--size`).
- Cache key uses the search query and all filters/options except `--from` and `--size`.
- Changing only `--from`/`--size` reuses cached ranked results and renews TTL by 1 minute.
- Without options, output stays in the current human-friendly tree format.

### 5) `watch`

Watches the project recursively and keeps indices synchronized in real time.

Usage:

```bash
idx watch
idx watch --show-updated-files
idx watch --debounce 500ms
```

Current behavior:

- Runs as a single process for the whole Git project (no need to run one watcher per directory).
- Detects file and directory changes recursively.
- Ignores `.git`, `.idx`, and paths ignored by `.gitignore`.
- Uses a short debounce window to batch frequent file events.
- Optional `--debounce <duration>` controls the event batch window (default: `750ms`).
- Reindexes only affected directories using the same sync logic.
- Optional `--show-updated-files` prints the deduplicated list of updated files in each synchronized batch.
- If the root index does not exist yet, creates the initial index automatically before entering watch mode.

Output examples:

```text
👀 Watch mode started. Press Ctrl+C to stop.
🔄 Synchronized 3 changed directorie(s).
  updated files:
  - internal/core/services/search/search_command_service.go
  - internal/core/services/indexing/watch_command_service.go
🛑 Watch mode stopped.
```

Supported `search` flags:

- `--format <text|json>`
- `--context <N>`
- `--json-pretty` (requires `--format json`)
- `--matches-only`
- `--files-only`
- `--from <N>`
- `--size <N>`
- `--path <pattern>` (repeatable)

Output format:

**Text format:**

- Shows a friendly header with match count: `📁 Found 42 file(s) matching your search`
- Includes pagination info when `--from` or `--size` is used
- Tree-like display with file paths and line numbers
- Each line prefixed with `├──` or `└──` for readability
- ANSI colors for file paths and line numbers (when output is terminal)

Example text output:

```text
📁 Found 2 file(s) matching your search
internal/adapters/repository/os_project_tree.go (score: 1.0000)
├── 19: func (tree OSProjectTree) CurrentDir() (string, error) {
└── 28: func (tree OSProjectTree) FindGitRoot(startDir string) (string, error) {

cmd/idx/main.go (score: 0.8500)
└── 42: projectTree := adapters.NewOSProjectTree()
```

**JSON format (`--format json`):**

- Returns structured object with `count` (total matches) and `results` array
- Each result includes `file`, `name`, `path`, `score`, and `matches` array
- Each match has `line` (number), `content` (full line text), and `match` (boolean flag)
- The `match` boolean flag distinguishes actual matches from context lines (`true` = matched term, `false` = context line)
- Pretty-printed with `--json-pretty` for human readability

Example JSON output (showing 1 file from a total of 2 matching files):

```json
{
  "count": 2,
  "results": [
    {
      "file": "./go.mod",
      "name": "go.mod",
      "path": "./go.mod",
      "score": 1,
      "matches": [
        {
          "line": 1,
          "content": "module idx",
          "match": true
        },
        {
          "line": 2,
          "content": "go 1.26",
          "match": false
        }
      ]
    }
  ]
}
```

**Key differences:**

| Feature | Text | JSON |
|---------|------|------|
| Match count header | ✅ Visual "📁 Found X" | ✅ `count` field with total |
| Match vs context indicator | ❌ Not indicated | ✅ `match: true/false` field |
| Human-readable | ✅ Yes | ❌ Machine-friendly |
| Structured | ❌ Visual format | ✅ Parseable objects |
| Colors (terminal) | ✅ ANSI codes | ❌ Plain text |

When no results are found:

**Text format:**
```text
No results found.
```

**JSON format:**
```json
{
  "count": 0,
  "results": []
}
```

### 5) `destroy`

Removes all `.idx` directories recursively from the project.

Usage:

```bash
idx destroy
```

Rules:

- Must run from the Git project root.
- Running outside the root returns an explicit error.

Success output:

```text
🧹 Index metadata removed from project.
```

### 6) `help` / `--help`

Shows command usage and available subcommands (provided by Cobra).

Usage:

```bash
idx help
idx --help
```

## Tokenization

Current tokenization rules:

- Lowercase normalization.
- Removes punctuation and splits by whitespace.
- Keeps letters, numbers, and `_` as token characters.
- Removes common English stop words.
- Removes tokens shorter than 2 characters.

Quick examples:

- `foo.bar` -> `foo`, `bar`
- `path/to/file` -> `path`, `to`, `file`
- `snake_case` -> `snake_case`

## Recommended workflow

1. In your Git project, run `idx init` once.
2. During development, use `idx search <terms>`.
3. Before pushing changes, run `make check`.
4. Build binaries with `make build` when needed.
5. Re-run `idx sync` whenever you want to regenerate existing indexes.
6. Run `idx destroy` to clean index metadata.

## Concurrency test profiles

The repository includes dedicated concurrency targets where `sync` writes while `search` reads:

```bash
make test-concurrency
make test-concurrency-race
make test-concurrency-ci
make test-concurrency-heavy
```

Notes:

- `test-concurrency-race` runs with `-race` and `RACE_COUNT` (default `3`).
- `test-concurrency-ci` uses a race-safe default workload to reduce timeout flakiness in CI.
- You can override workload via env vars, for example:

```bash
IDX_CONCURRENCY_TIMEOUT_SECONDS=60 RACE_COUNT=4 make test-concurrency-ci
```

## Benchmark

`idx` vs `grep` comparison:

- File: `docs/benchmarks/search-vs-grep.md`
- Command:

```bash
go test ./internal/core/services/search -run '^$' -bench BenchmarkSearchVsGrep -benchmem
```

## Indexing logs

During `idx init` and `idx sync`, each indexed directory writes file-level indexing logs under:

```text
<directory>/.idx/logs/
```

Files:

- Active log: `tlog.idx`
- Rotated logs: `tlog_YYYYMMDDHHMMSS.log`

Rotation policy:

- Rotate when active log reaches/exceeds `1MB`
- Keep up to `5` rotated logs per directory

Standard log line format:

```text
path=<file-path>\thash=<sha256>\tindexed_at=<RFC3339-UTC>
```

Example:

```text
path=/repo/internal/app/service.go	hash=0d7e7f4b4ddf0f6f8d3d6317086f8c7d3f5ab5a889a2c7e9a6f43c1ab4d74f7d	indexed_at=2026-04-26T14:22:30Z
```

## Common errors

No command:

```text
missing command: got [...], expected one of [sync init inspect destroy search]
```

Invalid command:

```text
unsupported command "<cmd>": expected one of [sync init inspect destroy search]
```

`search` without terms:

```text
missing search query: got [...], expected idx search <terms>
```
