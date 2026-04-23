# ADR 0001: Adopt BM25 Per-Directory Inverted Index

## Status

Accepted

## Context

The `idx index` command originally generated `.idx/index.idx` as a plain text list of entries in each directory. That format was cheap to write, but it did not support ranked search and did not capture document content.

The CLI runs locally on the user's machine and indexes source trees that can be large. Search quality and memory footprint both matter.

## Decision

The project uses a BM25-based inverted index built from file contents.

The index follows these rules:

- The index is built from files in the current directory.
- `.gitignore` rules are respected during indexing.
- Subdirectory names are not inserted as searchable items in the parent index.
- Each subdirectory gets its own `.idx/index.idx` file.
- Empty directories still receive an empty index file.

## Rationale

- BM25 provides relevant ranking without requiring external dependencies.
- Per-directory indices keep memory use bounded for local search workflows.
- Excluding subdirectory names avoids mixing navigation metadata with searchable document content.
- Respecting `.gitignore` avoids indexing generated or vendor content that is not useful to search.

## Consequences

### Positive

- Search can use term frequency and inverse document frequency.
- Indices are smaller and more focused because each directory is isolated.
- The model matches the repository layout and keeps traversal predictable.

### Negative

- Search across the full project must aggregate multiple directory-level indices.
- Index generation is more expensive than writing a plain text file list.

## Implementation Notes

- Domain structures live under `internal/core/domain`.
- Indexing logic lives under `internal/core/services`.
- Repository implementations live under `internal/adapters/repository`.