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

- Keep your own comments. Don't strip them on refactor — they carry
  intent and provenance.
- Write WHY, not WHAT. Skip `// increment counter` above `i++`.
- Docstrings on public functions: intent + one usage example.
- Reference issue numbers / commit SHAs when a line exists because
  of a specific bug or upstream constraint.

## Tests

- Tests run with a single command: `<project-specific>`.
- Every new function gets a test. Bug fixes get a regression test.
- Mock external I/O (API, DB, filesystem) with named fake classes,
  not inline stubs.
- Tests must be F.I.R.S.T: fast, independent, repeatable,
  self-validating, timely.

## Dependencies

- Inject dependencies through constructor/parameter, not global/import.
- Wrap third-party libs behind a thin interface owned by this project.

## Structure

- Follow the Go project layout convention (`https://go.dev/doc/modules/layout`) and Hexagonal Architecture principles.
- Prefer small focused modules over god files.
- Predictable paths:
```
.
├── cmd/
│   └── api/
│       └── main.go          # Dependency injection & app startup
├── internal/
│   ├── core/                # The "Inside" (Business Logic)
│   │   ├── domain/          # Entities/Models (structs)
│   │   ├── ports/           # Interfaces (contracts for In/Out)
│   │   └── services/        # Logic implementation (Use cases)
│   └── adapters/            # The "Outside" (Infrastructure)
│       ├── handlers/        # Input (HTTP, gRPC, CLI)
│       └── repository/      # Output (Postgres, Redis, S3)
└── go.mod
```

## Formatting

- Use the language default formatter (`gofmt`). Don't discuss style beyond that.

## CLI Output

- Generated prompts must be user-friendly with colors and easy to understand.
- Use formatting (bold, colors, spacing) to highlight key information.
- Keep messages concise and actionable.
- Prefer emoji and visual hierarchy over plain text.

## Logging

- Plain text only for user-facing CLI output.