
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

- Tests run with a single command: `make test`.
- Every new function gets a test. Bug fixes get a regression test.
- Use `go.uber.org/mock` for interface-based mocks in tests.
- Generate mocks with `mockgen` when needed; avoid handwritten fake/mock
  implementations for collaborators that are already expressed as ports.
- Tests must be F.I.R.S.T: fast, independent, repeatable,
  self-validating, timely.

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
│   │   └── output/          # Writer interface
│   └── app/
│       └── cli/             # Cobra commands — no business logic
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