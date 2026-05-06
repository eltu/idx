# ADR 0008: Search Boolean Operator (AND / OR) and AND Relaxation

## Status

Accepted

## Context

The BM25 search engine previously required **all** query terms to appear in a
document before scoring it. This implicit AND behaviour was good for precision
but limited recall: searching for `err := root.Execute()` with five tokens
would only return files containing every single token, missing files that were
clearly relevant but used slightly different phrasing.

Users with exploratory queries — e.g. looking for files related to two
loosely-coupled concepts — had no way to broaden the result set short of
running multiple independent searches and merging results manually.

Even after introducing explicit `AND` and `OR`, a second precision-oriented
workflow remained awkward: very specific `AND` queries with four or more terms
would often return zero results because the last one or two tokens were too
literal, even when the repository contained files matching the earlier, more
structural portion of the query. Users wanted a way to keep AND-style ranking
semantics while progressively relaxing the least important trailing tokens.

Two additional problems were identified with the pre-existing AND-only
implementation:

1. **Score ties broke alphabetically** — BM25 scores are normalised per
   directory to `[0, 1]`, so the highest-scoring file in each directory always
   receives `1.0`. When several files tied at `1.0`, the tiebreaker was
   lexicographic path order. This meant `cmd/idx/main.go` could beat
   `internal/adapters/handlers/cli/command_runner.go` even when the latter
   contained the exact phrase being searched.

2. **Partial-match penalty for OR was missing** — a naive OR that simply
   unions the document sets would let a file matching only one of five terms
   (but with a very high TF for that term) outscore a file matching all five
   terms.

## Decision

### 1. Introduce `--operator` flag

Add a `--operator` flag to `idx search` with two accepted values:

- `AND` (default) — document must contain **all** query terms.
- `OR` — document must contain **at least one** query term.

The flag is validated at the CLI layer; unsupported values return a descriptive
error. The value is propagated through `ports.SearchOptions.Operator` to the
core service and scoring functions, keeping the CLI adapter decoupled from
ranking logic.

### 2. Term-coverage multiplier for OR

To prevent high-TF single-term documents from outranking full-match documents
in OR mode, each document's BM25 score is multiplied by its **term coverage
fraction**:

```
finalScore = bm25Score × (matchedTermCount / totalQueryTermCount)
```

A document matching all N terms receives a multiplier of `1.0` (no penalty).
A document matching only k < N terms receives `k/N`, ensuring full-match
documents always rank above partial-match documents with equal or lower raw
BM25. This multiplier is only applied in OR mode; AND mode is unchanged
because every matched document already satisfies full coverage by definition.

### 3. Term-concentration tiebreaker

After BM25 + coverage, documents within the same directory still normalise to
`1.0`. A second tiebreaker was added: **term concentration** — the maximum
number of distinct query terms that co-occur on a single matched line.

Sort key priority (descending):

1. Normalised BM25 score
2. Term concentration (more terms per line = higher rank)
3. Lexicographic file path (stable, deterministic)

This ensures that a file containing the exact phrase `err := root.Execute()`
on one line ranks above files where the same tokens appear on separate,
unrelated lines.

### 4. AND relaxation via trailing-term fallback

Add an optional `--relaxation >N` flag for `idx search` that is only valid
with `--operator AND`.

When enabled for queries with more than three unique terms, search evaluates a
sequence of decreasing AND prefixes by removing terms from right to left:

- full query: `t1 t2 t3 t4 t5`
- fallback 1: `t1 t2 t3 t4`
- fallback 2: `t1 t2 t3`

Fallback stops once the candidate prefix would have `N` or fewer terms.
For example, `--relaxation '>2'` allows prefixes of length `3+` only.

Results from all evaluated prefixes are merged per document, keeping the best
variant for that document. Ranking prioritises:

1. Matched term count (more matched query terms = higher rank)
2. Normalised BM25 score
3. Term concentration
4. Lexicographic file path

This preserves a precision-first search experience while still surfacing
relevant near-miss results when the strict full query would otherwise return
nothing.

### 5. Nil-safety for proximity bonus in OR mode

The existing proximity bonus assumed all matched documents contained every
query term. With OR, a document may be matched through only one term, so
`index.Terms[term].Docs[filePath]` could be `nil` for the other terms.
A nil guard was added to `minimumDistanceForTermPair` to skip pairs where
either term is absent from the document.

## Consequences

- **Precision vs recall**: AND remains the default to preserve existing
  precision-first behaviour. OR and relaxation are opt-in.
- **No index changes**: both operators work on the existing inverted index
  structure; no re-indexing is required.
- **Relaxation is bounded**: fallback only applies to queries with more than
  three terms and never reduces the candidate prefix to `N` terms or fewer.
- **Score semantics for strict AND remain unchanged**: the coverage multiplier
  is still OR-only. Relaxed AND adds a matched-term-count rank key only when
  relaxation is active.
- **Test coverage**: regression tests were added for the coverage multiplier,
  the concentration tiebreaker, AND relaxation ranking, and the CLI flag
  validation.
