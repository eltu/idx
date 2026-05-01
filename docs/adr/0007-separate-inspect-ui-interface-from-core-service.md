# ADR 0007: Separate Inspect UI Interface from Core Service

## Status

Accepted

## Context

The inspect flow was implemented with UI execution details coupled to
`InitCommandService` internals. This created architectural friction:

- Core service methods depended on direct UI execution wiring.
- Inspect orchestration mixed business/service concerns with presentation concerns.
- Evolving UI adapters (CLI/TUI) increased risk of touching core behavior.
- Tests for service internals became harder to isolate from UI execution paths.

The project architecture requires clear ports/adapters boundaries and small,
focused modules.

## Decision

Introduce an explicit inspect UI port in core and inject its implementation
through dependency injection.

### Contract

Add `ports.InspectUIRunner`:

- Method: `Run(index *domain.InvertedIndex) error`
- Responsibility: execute inspect user interface for a preloaded index.

### Service Integration

Update `InitCommandService`:

- Add `inspectUI ports.InspectUIRunner` dependency.
- Validate this dependency in `validateDependencies()`.
- Replace direct inspect UI execution with `service.inspectUI.Run(...)`.

Provide constructors:

- `NewInitCommandService(...)` remains as compatibility constructor.
- `NewInitCommandServiceWithInspectUI(...)` enables explicit adapter injection.

### Adapter Boundary

Add TUI adapter implementation in handlers layer:

- `internal/adapters/handlers/tui/inspect_runner.go`
- Implements `ports.InspectUIRunner` and delegates to inspect UI execution.

Wire adapter from bootstrap (`cmd/idx/main.go`) using dependency injection.

## Alternatives Considered

### Keep direct inspect UI call inside service

- Pros: fewer files, minimal changes.
- Cons: violates core/adapters separation and increases coupling.

### Move all inspect UI files immediately to adapters in one big change

- Pros: cleaner final structure in one step.
- Cons: high migration risk and larger regression surface.

### Add a function callback only in tests

- Pros: minimal production changes.
- Cons: no architectural boundary in production code.

## Consequences

### Positive

- Core service is decoupled from inspect presentation implementation details.
- UI execution becomes replaceable (different TUI/CLI adapters).
- Better testability through explicit dependency injection.
- Incremental migration path toward full inspect UI module split.

### Negative

- Adds one more dependency to `InitCommandService`.
- Requires test fixtures/build wiring to provide `InspectUIRunner`.
- Temporary dual constructor maintenance during migration.

### Trade-offs

- Chose incremental extraction through a port first to reduce risk.
- Accepted temporary compatibility constructor to avoid broad call-site churn.

## Implementation Notes

- Core port added at `internal/core/ports/inspect_ui_runner.go`.
- Service constructor and dependency validation updated in
  `internal/core/services/indexing/init_command_service.go`.
- Adapter implementation added at
  `internal/adapters/handlers/tui/inspect_runner.go`.
- Bootstrap wiring updated in `cmd/idx/main.go`.

This ADR records the boundary decision. Further UI file decomposition for inspect
(model/update/render split) is tracked as follow-up refactor work.