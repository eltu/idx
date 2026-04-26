# ADR 0004: Use checksum-based incremental sync

Date: 2026-04-24

## Status
Accepted

## Context
The sync command currently rebuilds every directory index on each run, even when file content has not changed. This increases indexing time and unnecessary disk writes.

The current search flow reads per-directory BM25 indices from .idx/index.idx and expects those files to keep the same structure and semantics.

## Decision
Sync now uses per-directory checksum metadata stored in .idx/checksum.idx.

For each eligible directory:
- Compute SHA-256 checksums for all allowed files.
- Compare current checksums with saved checksums.
- If checksums are unchanged and index file exists, skip reindexing.
- If checksums changed, index is missing, or checksum file is missing, rebuild the directory BM25 index and persist new checksums.

The checksum map key is the file name inside each indexed directory, and the value is the file content SHA-256 hash.

## Consequences
Positive:
- Reduces unnecessary index rebuilds when files are unchanged.
- Keeps search command behavior unchanged because index format and lookup paths remain the same.
- Detects added, changed, removed, and renamed files through checksum map comparison.

Trade-offs:
- Sync still reads file content to compute checksums.
- Full per-file incremental BM25 update is not implemented; when a directory changes, that directory index is rebuilt.

## Notes
This decision preserves compatibility with existing search behavior and output while improving sync efficiency.
