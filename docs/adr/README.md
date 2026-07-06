# Architecture Decision Records

This directory contains the Architecture Decision Records (ADRs) for idx. Each ADR captures a significant design decision: the context, the options considered, the choice made, and the consequences.

New ADRs are added sequentially. Superseded decisions are noted in the relevant ADR body.

## Index

| # | Decision |
|---|---|
| [0001](0001-adopt-bm25-per-directory-index.md) | Adopt BM25 Per-Directory Inverted Index |
| [0002](0002-use-binary-gob-index-serialization.md) | Use Binary GOB Index Serialization |
| [0003](0003-separate-metadata-filters-from-bm25-content.md) | Separate Metadata Filters from BM25 Content |
| [0004](0004-use-checksum-based-incremental-sync.md) | Use Checksum-Based Incremental Sync |
| [0005](0005-add-realtime-watch-index-sync.md) | Add Realtime Watch-Based Index Synchronization |
| [0006](0006-add-daemon-management-system.md) | Add Daemon Management System |
| [0007](0007-separate-inspect-ui-interface-from-core-service.md) | Separate Inspect UI Interface from Core Service |
| [0008](0008-search-boolean-operator-and-or.md) | Search Boolean Operator (AND / OR) and AND Relaxation |
| [0009](0009-filename-partial-match-bonus-for-ranking.md) | Filename Partial-Match Bonus for Relevance Ranking |
| [0010](0010-index-filename-tokens-in-bm25-corpus-for-recall.md) | Index Filename Tokens in BM25 Corpus for Recall |
| [0011](0011-destroy-disables-daemon-before-removing-indices.md) | `idx destroy` Disables Daemon Before Removing Indices |
| [0012](0012-add-search-extension-metadata-filter.md) | Add File-Extension Metadata Filter to `idx search` |
| [0013](0013-skills-install-command.md) | Skills Install Command |
| [0014](0014-project-level-config-file.md) | Project-Level Configuration File (`.idx.yml`) |
| [0015](0015-parallel-directory-indexing.md) | Parallel Directory Indexing with Bounded Concurrency |
| [0016](0016-read-command-and-access-log.md) | Read Command and File Access Log |
| [0017](0017-read-popularity-search-boost.md) | Read Popularity Boost for Search Ranking |
| [0018](0018-modularizacao-por-feature.md) | Modularization by Feature Package |
| [0019](0019-ipc-jsonrpc-unix-socket.md) | IPC via JSON-RPC 2.0 over Unix Socket |
| [0020](0020-server-as-self-managing-daemon.md) | Server as Self-Managing Daemon with Embedded Watch |
| [0021](0021-remove-idx-watch-command.md) | Remove `idx watch` Command |
| [0022](0022-idx-init-bootstrap-exception.md) | `idx init` Executes In-Process as a Bootstrap Exception |
| [0023](0023-skills-project-enforcement-and-skill-asset-drift-detection.md) | Skills Project-Level Enforcement and Skill Asset Drift Detection |
| [0024](0024-related-command-co-read-term-overlap.md) | `idx related` — Co-Read Affinity + BM25 Term Co-Occurrence |
| [0025](0025-related-signals-git-cochange-persistent-coread.md) | `idx related` — Git Co-Change + Persistent Co-Read Matrix |
| [0026](0026-destroy-stops-watch-loop-before-removing-indices.md) | `idx destroy` Stops the Watch Loop Before Removing Indices (supersedes 0011) |
