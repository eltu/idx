# config

## Purpose

Show and inspect the resolved project configuration sourced from `.idx.yml` at the Git project root.

## Usage

```bash
idx config
idx config show
```

## Subcommands

| Subcommand | Description |
| --- | --- |
| `show` | Display the full resolved configuration table and flag which values come from `.idx.yml` |

## Arguments

- `config` (no subcommand): displays usage.
- `show`: no positional arguments.

## Flags

- Global: `--quiet`, `-q`.

## Behavior and Side Effects

- Read-only command; no filesystem writes.
- Config is resolved from `.idx.yml` at the Git project root.
- Precedence chain: built-in defaults → `.idx.yml` → CLI flags.
- All 14 configurable keys are displayed with their resolved value, source, and original default for overridden keys.

## Output — `idx config show`

When `.idx.yml` exists:

```
  Config  /home/user/my-project/.idx.yml

  search.format         json     ← .idx.yml   (default: text)
  search.size           0        · default
  search.operator       AND      · default
  search.context        0        · default
  search.relaxation              · default
  search.cache_ttl      1m0s     · default
  search.max_workers    4        · default
  watch.debounce        750ms    · default
  index.ignore          []       · default
  bm25.k1               1.5      · default
  bm25.b                0.75     · default
  bm25.proximity_weight 3        · default
  bm25.popularity_weight 0.3     · default
  log.level             error    · default
```

- Keys set in `.idx.yml`: rendered bold indigo with source `← .idx.yml  (default: <value>)`.
- Keys at built-in defaults: muted with source `· default`.

When no `.idx.yml` exists:

```
  No .idx.yml found — using built-in defaults.
  Tip: create .idx.yml at the project root to customize defaults.
```

## Errors

- No command-specific runtime errors.

## Reference — All Configurable Keys

| Key | Type | Default | Notes |
| --- | --- | --- | --- |
| `search.format` | string | `text` | `text` or `json` |
| `search.size` | int | `0` | `0` = unlimited |
| `search.operator` | string | `AND` | `AND` or `OR` |
| `search.context` | int | `0` | Context lines around matches |
| `search.relaxation` | string | `""` | Format `>N`; activates AND relaxation |
| `search.cache_ttl` | duration | `1m` | BM25 result cache TTL (e.g. `30s`, `5m`) |
| `search.max_workers` | int | `4` | Parallel index-load workers |
| `watch.debounce` | duration | `750ms` | Debounce window for file events |
| `index.ignore` | list | `[]` | Glob patterns to exclude from indexing |
| `bm25.k1` | float | `1.5` | BM25 term-frequency saturation |
| `bm25.b` | float | `0.75` | BM25 document-length normalization |
| `bm25.proximity_weight` | float | `3.0` | BM25 proximity bonus weight |
| `bm25.popularity_weight` | float | `0.3` | Read-popularity boost weight; `0` disables the boost |
| `log.level` | string | `error` | `debug`, `info`, `warn`, `error` |

## `.idx.yml` Format

```yaml
# .idx.yml — place at the Git project root
search:
  format: json        # text | json
  size: 20            # 0 = unlimited
  operator: AND       # AND | OR
  context: 0
  relaxation: ""      # "" = off | ">3" = relax when >3 terms
  cache_ttl: 1m
  max_workers: 4

watch:
  debounce: 750ms

index:
  ignore:
    - vendor/
    - "*.pb.go"

bm25:
  k1: 1.5
  b: 0.75
  proximity_weight: 3.0
  popularity_weight: 0.3  # 0 = disabled

log:
  level: error        # debug | info | warn | error
```

Notes:
- Duration values must be strings (e.g. `250ms`, `2m`, `30s`). Invalid durations fail at startup.
- `index.ignore` values are glob patterns matched against relative paths from the project root.
- `log.level` is also overridable via the `IDX_LOG_LEVEL` environment variable; env takes precedence over `.idx.yml`.
- Only project-level config is supported. There is no global `~/.idx/config.yml`.

## Examples

```bash
idx config show
```
