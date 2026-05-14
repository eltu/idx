# ADR 0014: Project-Level Configuration File (.idx.yml)

## Status
Accepted

## Context
All idx settings were either hardcoded constants or required explicit CLI flags on every
invocation. This forced users to repeat the same flags across runs and made it impossible
for different projects to have different default behaviors (e.g. a project that always wants
JSON output, or a monorepo that needs a larger watch debounce window).

A configuration file was needed to let projects declare their own defaults without changing
the global binary behavior.

## Decision

**Single project-level file: `.idx.yml` at the Git root.**
The file lives at the project root (resolved via `git rev-parse --show-toplevel`) and is
version-controlled alongside the project. There is no global `~/.idx/config.yml`. A global
file would silently affect every project on the machine, making behavior harder to reproduce
across team members and CI environments.

**Format: YAML with Go duration strings.**
YAML is human-readable, widely understood, and requires no schema registry. Duration fields
(`cache_ttl`, `debounce`) are expressed as Go duration strings (e.g. `750ms`, `2m`) parsed
via `time.ParseDuration`. This avoids inventing a custom unit system and keeps the format
consistent with CLI flag syntax.

**Override detection via pointer fields.**
The YAML adapter uses an intermediate struct (`yamlIdxConfig`) with pointer fields
(`*string`, `*int`, `*float64`) for every configurable key. A nil pointer means the key was
absent from the file; a non-nil pointer means it was explicitly set. This produces a precise
list of overridden keys (`[]string`, e.g. `"search.format"`, `"bm25.k1"`) that is threaded
through the system for UX purposes without any string diffing or reflection.

**Precedence chain: defaults → `.idx.yml` → CLI flags.**
Built-in defaults live in `domain.DefaultIdxConfig()`. The YAML adapter merges explicit file
values over those defaults. CLI flags always take precedence over both. This means any flag
passed at the command line still overrides the project config, keeping existing scripts
unaffected.

**Architecture: port + adapter, consistent with other repositories.**
A `ConfigRepository` port in `core/ports/` defines `Load(projectRoot string)` and
`FilePath(projectRoot string)`. The YAML implementation lives in
`adapters/repository/yaml_config_repository.go`. The domain struct `IdxConfig` and
`DefaultIdxConfig()` live in `core/domain/config.go`. Services receive tuning values
through existing constructor options and the new `WithTuning` builder on
`SearchCommandService`; no service imports the config adapter directly.

**BM25 constants promoted to `searchTuning` struct.**
The five search constants (`bm25K1`, `bm25B`, `proximityWeight`, `maxSearchWorkers`,
`searchCacheTTL`) were package-level constants in the search service. They were converted to
fields on a `searchTuning` struct and exposed via `WithTuning(SearchServiceOptions)` so the
wiring layer can pass values from the loaded config without touching service internals.
Standalone scoring functions (`scoreDocuments`, `addTermScores`, `proximityBonusForDocument`)
receive `searchTuning` as a parameter rather than closing over package state.

**Early log-level load before the DI graph.**
The logger is created before `run()` sets up the full dependency graph. To respect
`log.level` from `.idx.yml`, `earlyLoadConfigLogLevel(projectRoot)` reads only that one key
from the file at startup. The `IDX_LOG_LEVEL` environment variable still takes precedence
over the file value.

**UX: config visibility in `idx status` and `idx config show`.**
Users need to know which config is active without reading raw files. Two surfaces address
this:
- `idx status` renders a rich overview panel that includes a `Config` row showing the
  filename and number of active overrides. The row is omitted when no `.idx.yml` exists to
  avoid noise on projects that have not adopted the file.
- `idx config show` prints a full table of all 13 keys with each value coloured by source:
  indigo for `.idx.yml` overrides (with the original default in parentheses) and muted grey
  for built-in defaults.

## Consequences

- Adding a `.idx.yml` to a project changes default flag values for all team members using
  that project; this is intentional and expected.
- Projects without `.idx.yml` are unaffected — `domain.DefaultIdxConfig()` reproduces the
  previous hardcoded behavior exactly.
- The `gopkg.in/yaml.v3` dependency was added to `go.mod`.
- Invalid duration strings in `.idx.yml` cause a startup error with a descriptive message
  rather than silently falling back to defaults, ensuring misconfigured files are caught
  early.
- New configurable keys can be added by: adding a field to `IdxConfig`, adding a pointer
  field to the matching `yaml*Config` struct, wiring it in the corresponding
  `apply*Overrides` helper, and adding a row to `buildConfigRows` in `config_commands.go`.
