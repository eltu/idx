
## Code style

- Functions: 4-20 lines. Split if longer.
- Files: under 500 lines. Split by responsibility.
- Cyclomatic complexity: must stay below 15 per function.
- One thing per function, one responsibility per module (SRP).
- Names: specific and unique. Avoid `data`, `handler`, `Manager`.
  Prefer names that return <5 grep hits in the codebase.
- Types: Must be explicit.
- No code duplication. Extract shared logic into a function/module.
- String literals used more than once must be extracted as named constants.
  Applies to any string with >5 characters that contains non-alphanumeric characters
  (e.g. command names, format strings, file paths, error messages, key bindings).
- Empty function bodies (including anonymous `func() {}`) must contain a comment
  explaining why they are intentionally empty (e.g. `/* no-op: reason */`).
- Cognitive complexity per function must stay below 15. When a closure or nested
  control structure pushes a function over the limit, extract the body into a
  named function or method.
- Functions and methods must have at most 7 parameters. When the limit is exceeded,
  group related parameters into a named struct (e.g. `FooDeps`, `BarContext`,
  `BazOutput`). Prefer grouping by cohesion: dependencies together, output channels
  together, contextual inputs together.
- Early returns over nested ifs. Max 2 levels of indentation.
- Exception messages must include the offending value and expected shape.
- Consecutive parameters of the same type must be grouped: `func foo(a, b string)` not `func foo(a string, b string)`.
- Single-method interfaces must follow the verb+"-er" naming convention (e.g. `Reader`, `Runner`, `Installer`).
  Exception: domain port interfaces may use descriptive compound names (e.g. `IndexRepository`, `InspectUIRunner`).
- Blank imports (`import _ "pkg"`) must have a comment explaining why the side-effect import is needed.

## Comments

- Keep existing comments. Don't strip them on refactor — they carry intent and provenance.
- Write WHY, not WHAT. Skip `// increment counter` above `i++`.
- Public functions must have a doc comment: one line of intent + one usage example.
- Reference issue numbers / commit SHAs when a line exists because of a specific bug or upstream constraint.

## Tests

- Every change must pass `make check` (runs `gofmt` + `golangci-lint` + `go test ./...`).
- Every new function gets a test. Bug fixes get a regression test.
- Tests must be F.I.R.S.T: fast, independent, repeatable, self-validating, timely.

### Naming

Use `Test<Type>_<Scenario>_<ExpectedResult>` with underscores separating segments and
PascalCase within each segment. The name must read as a sentence — "When X, it should Y":

```
TestReadLogRepository_RecordRead_CreatesEntryOnFirstRead
TestSearchCommandService_Run_ReturnsErrorWhenDependenciesAreNil
TestBinaryIndexRepository_SaveIndex_ReturnsErrorForInvalidDirectory
```

Never use generic names like `TestError`, `TestService`, or camelCase without underscores.

### Structure — Arrange / Act / Assert

Every non-trivial test must have the three sections with comments:

```go
func TestDestroyCommandService_Run_RemovesIdxDirectories(t *testing.T) {
    t.Parallel()

    // Arrange
    tree := newFakeProjectTree(rootDir, rootDir)
    service := lifecycle.NewDestroyCommandService(tree, output)

    // Act
    err := service.Run()

    // Assert
    require.NoError(t, err)
    assert.Len(t, tree.removed, 3)
}
```

### Parallelism

Add `t.Parallel()` as the **first statement** in every test that is isolated
(uses only local variables, `t.TempDir()`, or mocks). Also add it inside each
`t.Run()` subtest. Never use `t.Parallel()` in tests that call `t.Setenv`,
`t.Chdir`, or any other function that mutates global process state.

### Assertions

Always use `testify/require` and `testify/assert`. Never use `t.Fatal` / `t.Fatalf`
/ `t.Error` / `t.Errorf` directly.

- Use `require` when the test cannot continue after a failure (error checks, nil guards).
- Use `assert` for non-blocking validations.
- Pass `expected` before `got`: `assert.Equal(t, expected, actual)`.
- Prefer specific assertions over `assert.True`: `assert.Len`, `assert.ErrorIs`,
  `assert.ErrorContains`, `assert.NotEmpty`, `assert.Positive`.

```go
// ✅
require.NoError(t, err)
assert.Equal(t, "AND", opts.Operator)
assert.Len(t, results, 3)
assert.ErrorIs(t, err, ErrNotInitialized)

// ❌
if err != nil { t.Fatalf("unexpected error: %v", err) }
assert.True(t, len(results) == 3)
```

### Table-Driven Tests

Convert 3 or more tests that cover the same function with different inputs into a
table-driven test. Capture the loop variable and parallelize each subtest:

```go
for _, tc := range tests {
    tc := tc
    t.Run(tc.name, func(t *testing.T) {
        t.Parallel()
        // ...
    })
}
```

### No time.Sleep

Replace `time.Sleep` with channels, context cancellation, or `sync.WaitGroup`.
When a test genuinely depends on real wall-clock timing (e.g. a debounce timer),
keep the sleep and add a comment explaining why it is unavoidable.

### Mocking

Use `go.uber.org/mock` for interface-based mocks. Generate with `mockgen`; avoid
handwritten fakes for collaborators already expressed as ports. Handwritten fakes
are acceptable for simple anonymous adapters (e.g. `fakeFileReader`).

### Identical method bodies (SonarQube S4144)

Two methods on the same type with identical bodies are a code smell. The linter
does not catch this — SonarQube reports it as S4144. When it happens, make the
second method delegate to the first:

```go
// ❌ identical bodies
func (w *captureWriter) WriteInline(text string) error {
    w.mu.Lock(); defer w.mu.Unlock()
    w.writes = append(w.writes, text)
    return nil
}

// ✅ delegate
func (w *captureWriter) WriteInline(text string) error {
    return w.WriteLine(text)
}
```

### Coverage gate

After every implementation, run `go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out`
and check the total coverage. If the result is **below 89%**, inspect the uncovered lines
(`go tool cover -html=coverage.out`) and add tests until the threshold is met.

Focus new tests on the uncovered paths that carry the most risk: error branches, edge cases,
and business rules — not trivial getters or generated code. Do not pad coverage with
low-value assertions just to hit the number.

### What to test

Prioritize: business rules, edge cases (zero / empty / max), every error path,
concurrency and cancellation, and regressions. High coverage does not mean high
quality — a test that only exercises the happy path has low value.

## Dependencies

- Inject dependencies through constructor/parameter, not global/import.
- Wrap third-party libs behind a thin interface owned by this project.

## Structure

- Follow the Go project layout convention (`https://go.dev/doc/modules/layout`) and Hexagonal Architecture principles.
- Prefer small focused modules over god files.
- Architecture decisions must be recorded under `docs/adr/` using sequential files like `0001-short-title.md`.
- When a change introduces a persistent technical decision or tradeoff, update an existing ADR or add a new one in `docs/adr/`.
- Root-level ad hoc decision documents are discouraged; decision records belong in `docs/adr/`.
- Predictable paths:
```
.
├── cmd/
│   └── idx/                 # Dependency injection & app startup
├── internal/
│   ├── features/            # Self-contained feature packages (domain + ports + service + storage)
│   │   ├── indexing/        # BM25 engine (domain, port, service, storage, tui)
│   │   ├── search/          # Search ranking pipeline
│   │   ├── daemon/          # Background process management
│   │   ├── read/            # File streaming + read log
│   │   ├── lifecycle/       # Destroy index
│   │   └── skills/          # Skills installation
│   ├── shared/              # Cross-feature concerns
│   │   ├── config/          # .idx.yml parsing
│   │   ├── filesystem/      # ProjectTree, FileReader, IgnoreMatcher
│   │   ├── ipc/             # Unix socket address + JSON-RPC method constants
│   │   ├── jsonrpc/         # Content-Length framing codec
│   │   └── output/          # Writer interface
│   └── app/
│       ├── cli/             # Cobra commands — no business logic
│       │   └── remote/      # RPC adapters: delegate CLI calls to idx server
│       ├── server/          # Unix socket server — accept loop + RPC handlers
│       └── tui/             # Terminal UI runners (inspect, progress)
└── go.mod
```

## Port Conventions

- Ports are plain Go interfaces defined in `port.go` within each feature package.
- Each port describes one capability (e.g. `InspectUIRunner`, `IndexRepository`).
- Implementations live alongside the interface in the same feature package or in a `storage/` sub-package.
- Features do not import Cobra or any delivery mechanism — CLI wiring lives in `internal/app/cli/`.

## Current Decisions

- ADR 0001: BM25 inverted index is generated per directory and indexes file contents only.
- ADR 0002: Index files are stored in binary GOB format by default to reduce disk and memory overhead.
- ADR 0003: File name and path are indexed as metadata filters separate from the BM25 content corpus.
- ADR 0004: Incremental sync uses checksums to avoid re-indexing unchanged directories.
- ADR 0005: Real-time watch mode syncs the index incrementally on file system events.
- ADR 0006: Daemon management system to run watch mode as a background process.
- ADR 0007: Inspect TUI is injected via the InspectUIRunner port to decouple UI from core service.
- ADR 0008: Search boolean operator (AND/OR) with term-coverage multiplier and term-concentration tiebreaker.
- ADR 0009: Filename partial-match bonus for relevance ranking.
- ADR 0010: Filename tokens are indexed in BM25 corpus for recall.
- ADR 0011: Destroy disables daemon before removing indices.
- ADR 0012: Search supports indexed metadata filtering by file extension (`--ext`).
- ADR 0013: Skills install command uses `git clone` (not curl-pipe) and explicit required editor argument.
- ADR 0014: Project-level `.idx.yml` at Git root; pointer-field YAML adapter for override detection; defaults → file → CLI flag precedence.
- ADR 0015: Parallel directory indexing with bounded concurrency (`runtime.NumCPU`) using a two-phase collect + index approach.
- ADR 0016: `idx read` command streams files via `bufio.Scanner`; access log at `.idx/read_log.idx` tracks read counts with 30-day TTL, inode-based rename detection, deletion pruning, and a 5-min write cache.
- ADR 0017: Read popularity boost in search ranking — additive log1p-normalised bonus with 14-day exponential decay; weight configurable via `bm25.popularity_weight` in `.idx.yml` and `--popularity-weight` CLI flag.
- ADR 0018: Codebase modularized by feature (`internal/features/<feature>/`); shared cross-cutting concerns in `internal/shared/`; CLI delivery in `internal/app/cli/`; features do not import Cobra.
- ADR 0019: IPC via JSON-RPC 2.0 over Unix socket (`~/.idx/<project>.sock`); persistent `idx server` holds index in memory; all index-related CLI commands (init, sync, status, search, read, inspect, destroy) are clients; no in-process fallback — missing server is a clear error.
- ADR 0020: `idx server` is a self-managing daemon (server + watch in one process); `idx server start/stop/status` manage the lifecycle; `idx daemon enable/disable` removed; DI cycle between InitCommandService and ServerDaemonService eliminated.
- ADR 0021: `idx watch` removed; watch loop is internal to `idx server run` via `WatchWithContext`; `watch.debounce` config key retained.

## Formatting

- Use the language default formatter (`gofmt`). Run with `make fmt`.
- Don't discuss style beyond that.

## CLI Output

- Generated prompts must be user-friendly with colors and easy to understand.
- Use formatting (bold, colors, spacing) to highlight key information.
- Keep messages concise and actionable.
- Prefer emoji and visual hierarchy over plain text.

## CLI rules

- Plain text only for user-facing CLI output (no ANSI in log lines, only in styled output blocks).