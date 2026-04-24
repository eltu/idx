# ADR 0003: Separate Metadata Filters from BM25 Content

## Status

Accepted

## Context

The index stores document path metadata. Users want to search by path, but adding metadata tokens to the same BM25 corpus as file content changes document length, term frequency, and inverse document frequency.

The project already relies on BM25 for ranked content search, and ADR 0001 defines the corpus as file contents. Path metadata serves a different purpose: it constrains the result set, but it should not influence relevance statistics.

## Decision

Path metadata is indexed in a separate metadata term map.

The BM25 corpus remains based on file content only.

Search accepts two independent inputs:

- content query for BM25 ranking
- path filter

Metadata filters reduce the candidate document set but do not change BM25 score, document length, term frequency, or IDF.

## Decision Drivers

- Preserve BM25 relevance for content search.
- Support navigation-oriented filtering by path.
- Keep metadata lookup explicit and predictable.
- Avoid coupling metadata tokens to content statistics.

## Consequences

### Positive

- Content ranking remains statistically stable.
- Users can filter by path without affecting BM25.
- Metadata-only searches are possible without inventing fake content scores.

### Negative

- Search now has separate ranking and filtering stages.
- Index files grow to include metadata term maps.
- Existing indices must be regenerated with `idx sync` or `idx init` to populate metadata filters.

## Operational Notes

- Content queries continue to use the BM25 `Terms` corpus.
- Path filters use `PathTerms`.
- Metadata-only searches may return results without matched content lines.