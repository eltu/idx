# ADR 0018: Modularização por Feature

## Status

Accepted

## Context

The IDX codebase was originally organized by **technical layer** (`core/domain/`, `core/ports/`, `core/services/`, `adapters/`). Each feature (indexing, search, daemon, read) was spread vertically across 4–5 directories. Adding a new capability required touching multiple packages with high context cost.

The project needs to support future extensions — semantic search, language-specific analyzers, alternative ranking strategies — without requiring changes across every layer.

## Decision

Reorganize the codebase by **feature**. Each capability lives in a self-contained package under `internal/features/<feature>/` with its own domain types, interfaces (ports), service, and storage. Cross-cutting concerns are isolated in `internal/shared/`. CLI delivery (Cobra) lives in `internal/app/cli/`, separate from business logic.

Target structure:

```
internal/
├── features/
│   ├── indexing/       # BM25 engine: domain, ports, service, storage, tui
│   ├── search/         # Ranking pipeline — extension point for semantic search
│   ├── daemon/         # Background process management
│   ├── read/           # File streaming + read log
│   ├── lifecycle/      # Destroy index
│   └── skills/         # Skills installation
├── shared/
│   ├── config/         # .idx.yml parsing (cross-feature)
│   ├── filesystem/     # ProjectTree, FileReader, IgnoreMatcher
│   └── output/         # Writer interface
└── app/
    └── cli/            # Cobra commands — no business logic
```

**Circular dependency between `indexing` and `daemon`** — `InitCommandService` needs to check daemon state; `DaemonService` needs to call init. Solved by defining a narrow `ProjectMonitorChecker` interface in `features/indexing/port.go`. `DaemonService` implements it without importing `features/indexing`. The DI cycle at wire-up is broken via `DaemonService.SetInitCommand()`.

**Phase 2 (future):** `internal/app/rpc/` will expose features as JSON-RPC over a Unix socket, enabling a thin CLI client to delegate to a running daemon. Zero changes to feature packages required.

## Consequences

- Adding semantic search requires only creating `features/search/semantic/` and registering it in `cmd/idx/main.go`.
- Each feature can be tested in isolation (`go test ./internal/features/search/...`).
- `cmd/idx/main.go` is the single wiring point — it imports from `features/*`, `shared/*`, and `app/cli/`.
- Old `internal/core/` and `internal/adapters/` are retained during migration and will be removed once all consumers have been updated.
- Features do not import Cobra or any delivery mechanism.
