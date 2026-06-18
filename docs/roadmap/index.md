# idx Roadmap

idx is a fast BM25 full-text search engine for Git repositories, designed to serve as the primary code retrieval primitive for AI coding agents. This roadmap reflects our priorities from v0.9.0 through v1.0.0.

## Guiding principles

**Local-first, agent-native.** Every feature should make idx more useful as the retrieval layer underneath an AI agent — not just as a developer CLI tool.

**Signal compounds over time.** The read log, popularity boost, and related-file graph are only valuable if agents consistently route through idx. A broken or outdated skill erodes the signal before it can accumulate.

**Precision over recall at agent scale.** Agents issue dozens of queries per session. Noisy results cost context window budget. Every ranking and filtering improvement has a multiplier effect.

**Distributed by design.** Codebases grow faster than local disk. The architecture must support remote index backends without compromising the local-first experience for small repos.

---

## Versions

| Version | Theme | Status |
|---|---|---|
| [v0.8.0](../releases/v0.8.0.md) | Stable CLI foundation | Released |
| [v0.9.0](v0.9.0.md) | AI integration layer | Planned |
| [v0.10.0](v0.10.0.md) | Git-aware primitives | Planned |
| [v0.11.0](v0.11.0.md) | Agent-optimized output | Planned |
| [v1.0.0](v1.0.0.md) | Multi-agent: worktree isolation + hybrid MCP search | Planned |

---

## Dependency map

```
v0.9.0  ──► v0.10.0: idx read populates the read log → idx related consumes it
v0.10.0 ──► v0.11.0: --since is the same mechanism as the branch delta index in v1.0.0
v0.11.0 ──► v1.0.0:  --chunk defines the result shape that remote MCP search also returns
```

---

## Out of scope

- **idx as an MCP server** — idx is an MCP client for remote backends, not a provider.
- **Embedding-based semantic search** — possible in a future cycle; BM25 + remote baseline resolves the scale problem without the operational cost of running a local embedding model.
