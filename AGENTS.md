
## Code style

- Functions: 4-20 lines. Split if longer.
- Files: under 500 lines. Split by responsibility.
- Cyclomatic complexity: must stay below 15 per function.
- One thing per function, one responsibility per module (SRP).
- Names: specific and unique. Avoid `data`, `handler`, `Manager`.
  Prefer names that return <5 grep hits in the codebase.
- Types: Must be explicit.
- No code duplication. Extract shared logic into a function/module.
- Early returns over nested ifs. Max 2 levels of indentation.
- Exception messages must include the offending value and expected shape.

## Comments

- Keep existing comments. Don't strip them on refactor — they carry
  intent and provenance.
- Write WHY, not WHAT. Skip `// increment counter` above `i++`.
- Docstrings on public functions: intent + one usage example.
- Reference issue numbers / commit SHAs when a line exists because
  of a specific bug or upstream constraint.

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
│   ├── core/                # The "Inside" (Business Logic)
│   │   ├── domain/          # Entities/Models (structs)
│   │   ├── ports/           # Interfaces (contracts for In/Out)
│   │   └── services/
│   │       ├── daemon/      # Daemon management use cases
│   │       ├── indexing/    # Init & BM25 indexing use cases
│   │       ├── search/      # Search use cases
│   │       └── lifecycle/   # Destroy use cases
│   └── adapters/            # The "Outside" (Infrastructure)
│       ├── handlers/
│       │   ├── cli/         # Input (CLI commands & adapters)
│       │   └── tui/         # Input (TUI implementation & adapters)
│       └── repository/      # Output (filesystem, index storage)
└── go.mod
```

## Port Conventions

- Ports live in `core/ports/` and are plain Go interfaces owned by this project.
- Each port describes one capability (e.g. `InspectUIRunner`, `IndexingRepository`).
- The default implementation of a port that belongs to a service lives alongside that service (e.g. `services/indexing/inspect_ui_runner.go`).
- Adapter implementations live in `adapters/` and wire the port to external infrastructure (e.g. `adapters/handlers/tui/`).

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

## Formatting

- Use the language default formatter (`gofmt`). Run with `make fmt`.
- Don't discuss style beyond that.

## CLI Output

- Generated prompts must be user-friendly with colors and easy to understand.
- Use formatting (bold, colors, spacing) to highlight key information.
- Keep messages concise and actionable.
- Prefer emoji and visual hierarchy over plain text.

## Logging

- Plain text only for user-facing CLI output. 