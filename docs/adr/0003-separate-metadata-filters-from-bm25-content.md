# ADR 0003: Separate Metadata Filters from BM25 Content

## Status

Accepted

## Context

The index stores file name and path for each document. Users want to search by these fields, but adding metadata tokens to the same BM25 corpus as file content changes document length, term frequency, and inverse document frequency.

The project already relies on BM25 for ranked content search, and ADR 0001 defines the corpus as file contents. File name and path serve a different purpose: they constrain the result set, but they should not influence relevance statistics.

## Decision

File name and path are indexed in separate metadata term maps.

The BM25 corpus remains based on file content only.

Search accepts three independent inputs:

- content query for BM25 ranking
- file name filter
- path filter

Metadata filters reduce the candidate document set but do not change BM25 score, document length, term frequency, or IDF.

## Decision Drivers

- Preserve BM25 relevance for content search.
- Support navigation-oriented filtering by file name and path.
- Keep metadata lookup explicit and predictable.
- Avoid coupling metadata tokens to content statistics.

## Consequences

### Positive

- Content ranking remains statistically stable.
- Users can filter by file name and path without affecting BM25.
- Metadata-only searches are possible without inventing fake content scores.

### Negative

- Search now has separate ranking and filtering stages.
- Index files grow to include metadata term maps.
- Existing indices must be regenerated with `idx sync` or `idx init` to populate metadata filters.

## Operational Notes

- Content queries continue to use the BM25 `Terms` corpus.
- File name filters use `FileNameTerms`.
- Path filters use `PathTerms`.
- Metadata-only searches may return results without matched content lines.