# ADR 0017: Read Popularity Boost for Search Ranking

## Status

Accepted

## Context

The search ranking pipeline uses BM25 + proximity bonus + filename match bonus.
Files that are frequently accessed via `idx read` are implicitly more relevant to
the developer's current context. The read log (`.idx/read_log.idx`) already tracks
per-file access counts and last-read timestamps (ADR 0016), making it available as
an additional ranking signal with no extra infrastructure.

## Decision

Add an additive **popularity bonus** to the search score, applied after per-directory
normalisation (same position as the filename bonus).

### Formula

```
decayFactor  = 0.5 ^ (daysSinceLastRead / 14)
raw          = clamp(log1p(readCount) / log1p(10) * decayFactor, 0, 1)
popularityBonus = raw * popularityWeight
```

Key properties:
- **`log1p` dampening** — prevents high read counts from dominating; ~10 reads maps
  to `raw = 1.0` before decay.
- **Exponential decay (half-life 14 days)** — recency matters; a file read 2 weeks
  ago contributes half the boost of a file read today.
- **Additive, capped at weight** — can never exceed `popularityWeight` (default 0.3),
  keeping BM25 and filename bonus dominant for lexical relevance.

### Configuration

`popularity_weight` under the `bm25` key in `.idx.yml` (default `0.3`).
Overridable per query via `idx search --popularity-weight <value>` (0 disables).

### Wiring

- `ports.ReadLogRepository` gains `LoadAll(projectRoot) ([]ReadLogEntry, error)`.
- `SearchCommandService.WithReadLog(repo)` injects the repository.
- `computeRankedResults` loads the popularity map once per search (keyed by
  absolute path) before spawning parallel directory workers.
- `buildSearchResult` applies `popularityBonus` alongside `fileNameMatchBonus`.

## Consequences

- Search results reflect developer access patterns in addition to lexical relevance.
- Files that have never been read via `idx read` are unaffected (bonus = 0).
- Projects without a read log degrade silently to the previous behaviour.
- The boost is tunable per project and per query; setting weight to 0 disables it.
