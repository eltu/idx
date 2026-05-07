---
title: idx — BM25 Code Search CLI
description: Fast code and text search for Git repositories, powered by BM25 and per-directory indexes.
layout: default
---

# idx

Fast code and text search CLI for Git repositories, powered by BM25 and per-directory indexes.

> ⚠️ This project is under active development and may contain bugs or breaking changes.

---

## What is idx?

`idx` builds a local BM25 inverted index for your Git project and lets you search your codebase from the terminal — fast, offline, and without sending data anywhere.

Unlike `grep` or `ripgrep`, `idx` ranks results by relevance: files whose content (and names) better match your query appear first. It handles multi-term AND / OR queries, CamelCase and snake_case file names, proximity bonuses, and filename-aware recall so that `search_scoring.go` is always returned when you search `scoring`.

It also supports metadata filters for both path and file extension, so you can constrain results to scopes such as `internal/core` or only `.go` files.

---

## Quick start

```bash
make build
./bin/idx init
./bin/idx search "auth token"
```

Or install and run:

```bash
go install ./cmd/idx
idx init
idx search "func main"
```

---

## Commands

| Command | Description |
|---|---|
| [`idx init`](features/init.md) | Create BM25 indexes for the current Git project |
| [`idx sync`](features/sync.md) | Resync indexes using checksum-based incremental update |
| [`idx status`](features/status.md) | Show index freshness and per-directory stats |
| [`idx search`](features/search.md) | Search indexed content with BM25 ranking |
| [`idx inspect`](features/inspect.md) | Interactively browse index contents |
| [`idx watch`](features/watch.md) | Keep indexes in sync in real time (foreground) |
| [`idx daemon`](features/daemon.md) | Manage background watch processes |
| [`idx destroy`](features/destroy.md) | Remove all index metadata |

---

## Search highlights

```bash
# Basic search
idx search "auth token"

# Broaden with OR
idx search auth token --operator OR

# AND with relaxation: search all 5 terms, fall back to 3 if no results
idx search func abc x y int --operator AND --relaxation '>2'

# Filter by path
idx search auth --path internal/core

# Filter by extension
idx search auth --ext go

# Combine path + extension filters
idx search auth --path internal/core --ext .go

# Structured output
idx search auth token --format json --json-pretty --explain

# Paginate
idx search auth token --from 10 --size 5
```

---

## Keep indexes fresh

**One-time:**
```bash
idx sync
```

**Realtime (foreground):**
```bash
idx watch
```

**Background daemon:**
```bash
idx daemon enable .
idx daemon status
idx daemon disable .
```

---

## Ranking

Results are ranked by a combination of:

1. **BM25 score** — term frequency × inverse document frequency, normalised per directory to `[0, 1]`.
2. **Filename match bonus** — files whose name contains an exact query token receive `+1.0`; substring match receives `+0.5`. CamelCase and snake_case are split before comparison.
3. **Term concentration** — tiebreaker based on how many distinct query terms co-occur on the same matched line.

> Filename tokens are also **indexed in the BM25 corpus**, so a file is always retrievable by its name even if the query term never appears in its content.

---

## Architecture decisions

| ADR | Decision |
|---|---|
| [ADR 0001](adr/0001-adopt-bm25-per-directory-index.md) | BM25 inverted index, generated per directory |
| [ADR 0002](adr/0002-use-binary-gob-index-serialization.md) | Binary GOB serialization for index files |
| [ADR 0003](adr/0003-separate-metadata-filters-from-bm25-content.md) | Metadata path filters separate from BM25 content corpus |
| [ADR 0004](adr/0004-use-checksum-based-incremental-sync.md) | Checksum-based incremental sync |
| [ADR 0005](adr/0005-add-realtime-watch-index-sync.md) | Real-time watch mode with filesystem events |
| [ADR 0006](adr/0006-add-daemon-management-system.md) | Daemon management for background watch processes |
| [ADR 0007](adr/0007-separate-inspect-ui-interface-from-core-service.md) | `InspectUIRunner` port to decouple TUI from core |
| [ADR 0008](adr/0008-search-boolean-operator-and-or.md) | Boolean operator (AND / OR) with term-coverage multiplier |
| [ADR 0009](adr/0009-filename-partial-match-bonus-for-ranking.md) | Filename partial-match bonus for relevance ranking |
| [ADR 0010](adr/0010-index-filename-tokens-in-bm25-corpus-for-recall.md) | Filename tokens indexed in BM25 corpus for recall |
| [ADR 0011](adr/0011-destroy-disables-daemon-before-removing-indices.md) | `idx destroy` disables daemon before removing indexes |
| [ADR 0012](adr/0012-add-search-extension-metadata-filter.md) | `idx search` adds indexed metadata filter by file extension (`--ext`) |

---

## Requirements

- Go `1.26+`
- Git repository (project root is resolved from `.git`)

---

## Benchmarks

- [idx vs grep benchmark](benchmarks/idx-vs-grep.md)

---

## Errors reference

Full list of error messages, causes, and remediation steps: [errors](features/errors.md).
