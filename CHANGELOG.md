# Release Notes — idx v0.1

**Release Date:** May 2026

---

## Overview

`idx` is a fast, local file indexing and search tool built around BM25 scoring. This first release establishes the full core feature set: index initialization, incremental sync, real-time watch, background daemon management, content search with metadata filtering, and interactive inspection.

---

## Features

### Core Indexing
- **BM25 indexing engine** with binary GOB serialization for fast, space-efficient index storage on disk.
- **Per-directory index** architecture — each directory maintains its own index, enabling efficient partial rebuilds.
- **Checksum-based incremental sync** — only directories whose content has changed are re-indexed, making `sync` operations fast on large projects.
- **Snapshot support** in the directory checksum repository, with metadata preservation across sync cycles.
- **Automatic removal** of directory indexes when no indexable files remain in a directory.
- **Symlink detection and filtering** in directory entry handling.

### CLI Commands
- **`idx init`** — initializes and builds the index for a project tree.
- **`idx sync`** — re-synchronizes the index for changed directories using checksum diffing.
- **`idx watch`** — monitors the filesystem in real time and automatically syncs the index on file changes, with configurable debounce timing and updated-files output.
- **`idx search`** — full-text BM25 search with:
  - Metadata filtering by file name and path (`--file`, `--path`).
  - Multiple file and path query support.
  - Pagination via `--from` and `--size` flags.
  - Result count header and pagination info in text output.
  - JSON output mode (`--json`) with structured response including total match count.
  - Context lines output (`--context`).
  - Files-only mode (`--files-only`).
  - Score explanation flag (`--explain`) for ranking metadata.
  - Search result caching for repeated queries.
  - Colored highlights in terminal output.
- **`idx inspect`** — interactive TUI for browsing and inspecting the current index state, with key-action management and log viewing.
- **`idx status`** — reports the indexing state of the project, detects unindexed directories, supports profiling output, and provides detailed error messages.
- **`idx destroy`** — removes the `.idx` directory and all index data for a project.
- **`idx daemon`** (subcommands) — background daemon management system for long-running index processes, with process spawner integration (`OSProcessSpawner`).
- Shell **completion** support added to the root command.

### Search Argument Parsing
- Leading reserved words and invalid options are treated as literal query terms.
- Robust edge-case handling for ambiguous queries.

### Tokenizer
- Whitespace-only tokenization (stop words removed) with comprehensive test coverage.

---

## Internal / Architecture

- Clean hexagonal architecture: `core/domain`, `core/ports`, `core/services`, `adapters/`.
- Mock generation for CLI command runner to enable isolated unit testing.
- Concurrency tests configurable via environment variable with CI workflow support.
- Cyclomatic complexity checks enforced via `golangci-lint`.
- ADRs documenting key architectural decisions:
  - ADR 0001: BM25 per-directory index
  - ADR 0002: Binary GOB index serialization
  - ADR 0003: Separate metadata filters from BM25 content
  - ADR 0004: Checksum-based incremental sync
  - ADR 0005: Real-time watch index sync
  - ADR 0006: Daemon management system
  - ADR 0007: Separate inspect UI from core service

---

## CI / Release Automation
- GitHub Actions workflow for automated releases with cross-platform binary builds.
- Dependabot configuration for Go module dependency updates.
- Security workflow with `govulncheck` and CodeQL analysis.
- All comments and output messages translated to English for consistency.

---

## License

Released under the **BSD 3-Clause License**.

---

## Known Limitations / Future Work
- No remote or networked index support.
- Index format may change in minor releases before v1.0.
