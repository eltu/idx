# ADR 0024: `idx related` — Co-Read Affinity + BM25 Term Co-Occurrence

## Status

Accepted

## Context

Users need a way to discover files related to what they are editing. Two signals are available without schema changes:

1. **Temporal co-read**: files that appear close together in the read log suggest contextual coupling.
2. **Term co-occurrence**: files that share significant vocabulary in the BM25 index are semantically related.

Additionally, `idx search` needed a way to restrict results to files changed since a given Git ref, enabling change-scoped queries.

## Decision

### `idx related <file>`

The command combines two signals in a weighted sum:

| Signal | Weight | Mechanism |
|--------|--------|-----------|
| Co-read affinity | 0.7 | Temporal proximity proxy in the read log |
| Term co-occurrence | 0.3 | BM25Score of target terms against all candidates |

**Co-read via temporal proximity (no schema change)**

Rather than tracking explicit sessions, files whose `LastReadAt` in the read log falls within ±2 hours of the target file's last read are treated as co-read. Score = `1/(1+deltaHours)`, decaying with distance. This avoids schema changes to `.idx/read_log.idx` while capturing the intent of session-based co-read.

**Two-phase term co-occurrence**

1. First pass: collect the target file's term vector (term → TF) from all indexes.
2. Second pass: for every other document, compute `BM25Score(targetTerms, doc)` using the collected terms as the query. Scores are normalized per-pass by max before combining.

**Fallback when data is insufficient**

If neither signal produces results, an empty list is returned with an informative message (`"No related files found."`). No error is raised.

**Output**

Text format shows ranked results with path, reason (`co-read`, `term-overlap`, or `both`), and score. JSON format serializes the same data for programmatic use.

### `idx search --since <ref>`

Git integration is done via subprocess (`exec.CommandContext`), the same pattern established in ADR 0013 for `git clone`. The command `git -C <projectRoot> diff --name-only <ref>...HEAD` returns files changed since the given ref. Results are post-filtered to include only changed files. Invalid refs surface a clear error including the offending value and git's stderr output.

## Consequences

- Co-read affinity is a proxy and degrades gracefully: when the read log is empty, only term overlap contributes.
- Temporal window (±2h) is hardcoded; future work could expose it via `.idx.yml`.
- Weights (0.7/0.3) are fixed constants; future work could make them configurable.
- The `--since` flag adds a git subprocess call per search; this is acceptable for interactive use but not for high-frequency automation.
- IPC types `RelatedRequest`, `RelatedResult`, `RelatedResponse`, and method `idx.related` are added to `internal/shared/ipc/protocol.go`.
