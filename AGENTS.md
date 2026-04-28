## Project Guidelines

- Never use `grep` or `rg` to search within the project. Use the project's own search solution.
- To search: `go run cmd/idx/main.go search "[TERM_TO_SEARCH]"`
- For a simple output that only shows file names, use `go run cmd/idx/main.go search "[TERM_TO_SEARCH]" --files-only`
- To filter by path without affecting the BM25 ranking of the content, use `go run cmd/idx/main.go search "[TERM_TO_SEARCH]" --path [PATH_FILTER]"`
- For metadata-only searches, use `go run cmd/idx/main.go search --path [PATH_FILTER]`
- If you need more information, prefer the output of the `--format json` parameter and the `--matches-only` flag for structured results. These two parameters return valid JSON containing the matching file, line number, and the full matching line.
- When more context is needed, add the `--context [NUMBER_OF_LINES]]` flag (see README for all available parameters).
## Code style

- Functions: 4-20 lines. Split if longer.
- Files: under 500 lines. Split by responsibility.
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
│   └── idx/
│       └── main.go                  # Dependency injection & app startup
├── internal/
│   ├── core/                        # The "Inside" (Business Logic)
│   │   ├── domain/                  # Entities/Models (structs)
│   │   ├── ports/                   # Interfaces (contracts for In/Out)
│   │   └── services/
│   │       ├── indexing/            # Init & BM25 indexing use cases
│   │       ├── search/              # Search use cases
│   │       └── lifecycle/           # Destroy use cases
│   └── adapters/                    # The "Outside" (Infrastructure)
│       ├── handlers/
│       │   └── cli/                 # Input (CLI commands)
│       └── repository/              # Output (filesystem, index storage)
└── go.mod
```

## Current Decisions

- ADR 0001: BM25 inverted index is generated per directory and indexes file contents only.
- ADR 0002: Index files are stored in binary GOB format by default to reduce disk and memory overhead.
- ADR 0003: File name and path are indexed as metadata filters separate from the BM25 content corpus.
- ADR 0004: Incremental sync uses checksums to avoid re-indexing unchanged directories.

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