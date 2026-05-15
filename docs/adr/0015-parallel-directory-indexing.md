# ADR 0015: Parallel Directory Indexing with Bounded Concurrency

## Status

Accepted

## Context

The `idx init` command builds per-directory BM25 indices by walking the project tree sequentially (DFS). Each directory is completely independent: it writes to its own `dir/.idx/index.idx` and `dir/.idx/checksum.idx`. On large projects this creates unnecessary latency.

## Decision

Index directories in parallel using a two-phase approach:

1. **Collect phase** — a lightweight DFS traversal (`collectAllDirectories`) that returns all eligible directory paths as `[]string`. No I/O beyond directory listing.
2. **Index phase** — `indexAllParallel` processes the list with `errgroup.WithContext` and `SetLimit(runtime.NumCPU())`.

The same parallel approach is applied to `syncEligibleDirectories`, which also benefits `idx sync` and the watch startup sync.

## Rationale

**Two phases instead of recursive fan-out**: parallelising `indexChildren` recursively with `errgroup.SetLimit` causes deadlock — goroutines at the limit try to submit children to the already-full pool while waiting for a slot. Separating collection from indexing eliminates this risk entirely.

**`runtime.NumCPU()` as the concurrency limit**: indexing is mixed CPU (BM25 tokenisation) and I/O (file reads). `NumCPU` saturates CPU without excessive I/O contention. This is conservative and safe on all hardware profiles.

**No coordination between workers**: each directory writes to its own isolated path. `BinaryIndexRepository.SaveIndex` already uses atomic writes (`tmpfile + os.Rename`). `DirectoryChecksumRepository` already has `sync.RWMutex`. No changes to production concurrency primitives were needed.

## Consequences

- `idx init` and `idx sync` are faster on multi-core machines with many directories.
- Test fakes (`fakeIndexRepository`, `fakeChecksumRepository`, `watchStartupIndexRepo`, `watchStartupChecksumRepo`) were updated with `sync.Mutex`/`sync.RWMutex` to match the goroutine-safe contract required by concurrent callers.
- `indexDirectory`, `indexChildren`, `subdirectoryEntries`, and `countEligibleDirectories` were removed; their responsibilities are now split between `collectAllDirectories` and `indexAllParallel`.
