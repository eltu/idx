# ADR 0012: Add File-Extension Metadata Filter to `idx search`

## Status

Accepted

## Context

`idx search` already supports metadata path filtering via `--path`, where
filtering is applied before result rendering and integrated with the ranking
pipeline. This is useful for narrowing scope by directory, but it does not
solve a common workflow need: restricting search to a file type across the
whole project (for example, only `.go` files).

Without an extension-aware filter in the index model, the command would need
to infer extension constraints by scanning all candidate document paths at
query time. That approach is more expensive, harder to combine with existing
metadata filters, and less consistent with the current indexed-metadata design
(ADR 0003).

The desired behavior is:

1. Allow users to pass an extension filter flag (example: `--ext go`).
2. Match files by extension regardless of input style (`go` or `.go`).
3. Apply filtering across all indexed project directories before final ranking
   and output formatting.

## Decision

Introduce a repeatable `--ext` flag for `idx search` and index file extensions
as dedicated metadata terms.

### 1. CLI contract

Add `--ext` as a repeatable search flag:

- `idx search query --ext go`
- `idx search query --ext .go`
- `idx search --ext go` (metadata-only search)

`--ext` participates in the same validation contract as other metadata
filters: a search is valid if it has query terms, or at least one metadata
filter (`--path` and/or `--ext`).

### 2. Index model

Extend `InvertedIndex` with `ExtensionTerms map[string]map[string]bool` and
populate it during indexing.

Normalization rule for stored extension terms:

- trim whitespace
- lowercase
- drop a leading `.`

Examples:

- `.GO` -> `go`
- `go` -> `go`
- empty extension remains empty and is ignored

### 3. Search pipeline integration

Apply extension filtering in the metadata matching stage, after path filtering
and before score filtering/ranking output.

Effective behavior:

- `metadataMatchedDocuments` computes all indexed docs
- applies `--path` constraints
- applies `--ext` constraints
- returns the intersection used by `filteredScores`

This keeps ranking semantics unchanged: BM25 still ranks the surviving
candidate set, and metadata-only queries still receive uniform scores.

### 4. Option normalization and cache key

Normalize extension filters in `normalizedSearchOptions` so equivalent inputs
map to the same internal representation.

Include extension filters in the search cache key to avoid cross-contamination
between cached results for different extension constraints.

## Decision Drivers

- **Developer workflow fit**: extension-scoped search is a frequent operation.
- **Performance and consistency**: indexed metadata filtering is cheaper and
  aligns with existing `--path` behavior.
- **Predictability**: `go` and `.go` should behave identically.
- **Composability**: extension filter must combine with path/query/operator
  options without changing BM25 semantics.

## Consequences

### Positive

- Users can restrict searches to a file type directly via `--ext`.
- Extension filtering works across all indexed directories in the project.
- Metadata-only extension search is supported (`idx search --ext go`).
- Cache behavior is correct for extension-specific queries.

### Negative

- Index metadata footprint increases slightly due to `ExtensionTerms`.
- Additional normalization and filter code increases complexity in search
  option handling and metadata matching.
- Files without extension are not matched by non-empty extension filters,
  which may surprise users expecting an "all source files" style shortcut.

## Alternatives Considered

1. Derive extension from `DocStats.Path` at query time only (no indexed terms).

Rejected because it would require full candidate scans for every extension
query and duplicate logic already solved by indexed metadata filters.

2. Reuse `PathTerms` for extension matching.

Rejected because path tokenization semantics are broader than extension
semantics and can introduce ambiguous matches; a dedicated metadata field is
clearer and more maintainable.

3. Add extension tokens into BM25 corpus only.

Rejected because extension is a hard filter concern, not a relevance signal.
Using BM25 terms alone would not provide strict file-type filtering.

## Operational Notes

Implementation points:

- CLI flag wiring and validation in
  `internal/adapters/handlers/cli/cobra_commands.go`
- Search option contract in `internal/core/ports/search_options.go`
- Extension metadata index structure in `internal/core/domain/inverted_index.go`
- Index population in `internal/core/services/indexing/bm25_index_service.go`
- Metadata filtering integration in
  `internal/core/services/search/search_metadata_filter.go`
- Option normalization and cache key update in
  `internal/core/services/search/search_result_options_output.go`

Test coverage includes:

- CLI wiring/validation for `--ext`
- metadata matching by extension
- option normalization for extension formats
- functional search behavior with extension filter
- indexing assertions for extension metadata population
