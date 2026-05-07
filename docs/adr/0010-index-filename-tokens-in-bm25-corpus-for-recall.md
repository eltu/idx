# ADR 0010: Index Filename Tokens in BM25 Corpus for Recall

## Status

Accepted

## Context

ADR 0003 established that file names and paths are stored as **metadata**
(`FileNameTerms` / `PathTerms`) and are used as post-retrieval filters rather
than BM25 content. This design kept the BM25 corpus clean and avoided
inflating IDF for common English words that happen to be popular file-name
fragments (e.g. `service`, `test`, `handler`).

However, it introduced a **recall gap**: if a query term appears only in a
file's name and not in its content, the file will not be included in the
candidate set at all — it simply won't be returned. For example, searching
for `scoring` would not return `search_scoring.go` if the word "scoring" does
not appear inside that file's content.

This violates a fundamental expectation: a developer who types `scoring`
should always find the file called `search_scoring.go`, regardless of whether
the word appears in its body.

Metadata-only storage (ADR 0003) remains appropriate for **path-level
filters** (`--path` flag), but is insufficient as the sole representation of
the file name in retrieval scenarios.

## Decision

Add a **third pass** to `BuildIndex` in `bm25_index_service.go` that
tokenises each document's filename via `domain.TokenizeFileName` and inserts
those tokens into the BM25 `Terms` map.

The pass follows the existing content-indexing passes and applies one rule to
avoid corrupting content statistics:

> If a term derived from the filename is already present in `Terms` for that
> specific document (i.e. the same term also occurs in the file's content),
> the filename-derived entry is **skipped** for that document.

This preserves:

- **IDF accuracy**: a term that genuinely appears in N files' content is not
  artificially boosted by filename occurrences in other files, since those
  only add the term for files where it was absent.
- **TF accuracy**: content frequency for a document is unchanged if the term
  already appears there.

The existing IDF ("fourth") pass runs after the new third pass and computes
IDF across the full populated `Terms` map, including filename-derived entries.

Tokenisation delegates to `domain.TokenizeFileName` (introduced alongside ADR
0009), which handles snake_case, CamelCase, dotted extensions, and path
separators uniformly.

```
// Third pass: index filename tokens for recall
for _, document := range documents {
    fileNameTokens := domain.TokenizeFileName(document.Name)
    fileNameFreqs, fileNamePositions := domain.CountTokenFrequencies(fileNameTokens)
    for term, freq := range fileNameFreqs {
        if termStats := index.Terms[term]; termStats != nil {
            if _, alreadyIndexed := termStats.Docs[document.Name]; alreadyIndexed {
                continue
            }
        }
        index.AddTerm(term, document.Name, freq, fileNamePositions[term])
    }
}
```

## Decision Drivers

- **Recall is non-negotiable**: a file named after the exact query term must
  always be retrievable.
- **Consistency**: the same `domain.TokenizeFileName` function used for
  ranking bonuses (ADR 0009) is reused here, so splitting behaviour is
  identical between retrieval and ranking.
- **Minimal IDF distortion**: the skip-if-already-indexed rule limits the
  impact on documents whose content already contains the term.

## Consequences

### Positive

- Files are now retrievable by their name tokens even when those tokens do
  not appear in their content.
- `idx search "scoring"` reliably returns `search_scoring.go`.
- Shared `TokenizeFileName` domain function keeps tokenisation consistent.

### Negative

- The BM25 corpus grows slightly: every unique filename token that does not
  appear in any file's content becomes a new term.
- IDF values for filename-derived terms reflect document frequency across the
  directory, which may be misleadingly high if many files share a common name
  prefix (e.g. `service`). This is the same tradeoff made by all full-text
  search engines that index metadata fields.
- Filename-derived entries have `freq=1` and positions derived from
  `TokenizeFileName`, so their BM25 contribution will generally be lower than
  a term that appears many times in content. Combined with the ranking bonus
  from ADR 0009, this produces reasonable end-to-end behaviour.

## Operational Notes

- `domain.TokenizeFileName` lives in `internal/core/domain/tokenizer.go` and
  handles CamelCase splitting via Unicode upper/lower transitions.
- Tests for the retrieval behaviour are covered by integration-style tests in
  `internal/core/services/search/`.
- ADR 0003 remains in effect for metadata-only path filtering; this ADR adds
  a parallel content-corpus entry and does not replace the metadata store.
