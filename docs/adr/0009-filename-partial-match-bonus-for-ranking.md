# ADR 0009: Filename Partial-Match Bonus for Relevance Ranking

## Status

Accepted

## Context

BM25 scoring is based entirely on term frequency and inverse document
frequency within a file's **content**. A file's name is not a factor in the
raw BM25 score, so a query like `func main` may rank files that mention the
word "main" frequently in their bodies above `main.go` or `main_test.go`,
even though those files are almost certainly the most relevant results.

This is a precision problem: developers intuitively expect files whose names
match query tokens to appear near the top of results, especially when the name
is an exact lexical match.

The challenge is that file names may use several naming conventions:

- Snake case: `bm25_index_service.go` → tokens: `bm25`, `index`, `service`, `go`
- CamelCase: `SearchScoringService` → tokens: `search`, `scoring`, `service`
- Dotted extensions: `main.go` → tokens: `main`, `go`

A simple substring check on the raw file name would fail for CamelCase and
produce false positives for short common tokens like `go`.

## Decision

After BM25 scoring and normalisation, apply an additive **filename match
bonus** to each result's score before final sorting.

The bonus values are:

| Match type | Bonus |
|---|---|
| Query term equals the full file stem (e.g. `main` matches `main.go`) | **+1.0** |
| Query term equals exactly one filename token after splitting | **+1.0** |
| Query term is a substring of one filename token | **+0.5** |
| No match | **0** |

Filename tokenisation is performed by `domain.TokenizeFileName`, which:

1. Splits the filename on `_`, `.`, `-`, and `/` boundaries.
2. Further splits each part by CamelCase word boundaries using Unicode
   upper/lower transitions.
3. Lowercases all tokens before comparison.

The exact-token check is evaluated before the substring check so that a query
term that fully equals a token always receives `1.0` rather than being
incorrectly matched as a substring of itself with `0.5`.

The bonus is applied in `search_command_service.buildSearchResult`:

```go
score: score + fileNameMatchBonus(terms, fileName),
```

The final score is not re-normalised after adding the bonus, so the bonus can
push a result above `1.0`. This is intentional: a file whose name is a
strong match should rank above files whose high BM25 score derives solely
from repeated term occurrences in content.

## Decision Drivers

- Developer intuition: files named after the search term should rank near the
  top.
- Non-destructive: the bonus is additive; content-heavy files with no name
  match are unaffected.
- CamelCase and snake_case must both be handled correctly without special-case
  logic per query.

## Consequences

### Positive

- `main.go` and `main_test.go` now rank near the top for a query of `func main`.
- CamelCase file names like `SearchScoringService.go` receive a bonus when
  querying `scoring` or `search`.
- Bonus logic is concentrated in one function (`fileNameMatchBonus`) with
  clear test coverage.

### Negative

- A file with a very high BM25 content score but an unrelated name may be
  displaced by a file with a lower content score but a matching name.
- The bonus magnitude (`1.0` / `0.5`) was chosen heuristically; different
  corpora may need tuning.

## Operational Notes

- `domain.TokenizeFileName` is shared with the indexing pipeline (ADR 0010)
  to keep tokenisation consistent between retrieval and ranking.
- Tests live in `internal/core/services/search/search_output_scoring_internal_test.go`.
