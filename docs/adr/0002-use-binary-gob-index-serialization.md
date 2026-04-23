# ADR 0002: Use Binary GOB Index Serialization

## Status

Accepted

## Context

The first BM25 implementation serialized indices as JSON. JSON made the file human-readable, but that is not the primary requirement for this CLI.

The CLI runs locally and prioritizes low memory use, compact index files, and fast load times. JSON adds text overhead, more allocations during parsing, and a larger on-disk footprint.

## Decision

The default index storage format is Go binary serialization using `encoding/gob`.

JSON is retained only as a secondary reference/debug format in code, and binary indices are the format written by `idx index`.

## Alternatives Considered

### JSON

- Pros: readable, portable, easy to inspect.
- Cons: larger files, slower parsing, higher memory overhead.

### GOB

- Pros: standard library, compact, type-safe, faster encode/decode for Go.
- Cons: not intended as a cross-language interchange format.

## Decision Drivers

- Minimize local disk usage.
- Reduce memory overhead when loading indices.
- Keep dependencies limited to the Go standard library.
- Preserve fast serialization and deserialization.

## Consequences

### Positive

- Observed reduction in index size of roughly 50% to 59% compared to JSON in local tests.
- Lower parsing overhead and fewer intermediate allocations.
- No external dependency or custom binary protocol is required.

### Negative

- Index files are no longer human-readable by default.
- External tools must use Go-aware decoding or a dedicated conversion utility.

## Operational Notes

- The default repository is `BinaryIndexRepository`.
- `idx index --debug` inspects the current directory binary index by decoding it and printing JSON.
- Re-running `idx index` regenerates indices in the binary format.

## Follow-Up Options

- Evaluate gzip compression if index size becomes a bottleneck again.
- Evaluate memory-mapped reads for very large indices.