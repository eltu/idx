# ADR 0013: Skills Install Command

## Status
Accepted (updated: embed strategy replaces git clone)

## Context
Users need a way to install idx skills into their editors (GitHub Copilot, Claude Code, Cursor)
directly from the CLI. The skills consist of two Markdown files (`SKILL.md` and
`references/idx-commands.md`) that are copied to the editor's skills directory.

## Decision
Introduce `idx skills install <editor>` as a new command under a new `Tools:` group.

**Fetch strategy: `//go:embed` (bundled distribution).**
Skill files are embedded directly into the binary at compile time. This approach:
- Requires no external dependencies (`git`, `curl`, `jq`, or network access)
- Guarantees that skills and the binary are always version-compatible
- Provides instant installation with no clone or download latency
- Works fully offline

A previous iteration used `git clone https://github.com/eltu/idx-skills` to fetch the
skills at install time. That approach was replaced because it required `git` in `$PATH`,
a network connection, and introduced the risk of version skew between the binary and skills.

**Claude permissions: Go stdlib JSON (`encoding/json`).**
For Claude, `~/.claude/settings.json` is updated to add `"Bash(idx *)"` to
`permissions.allow`. This is handled with `encoding/json` — no `jq` dependency.
Existing fields in `settings.json` are preserved. The permission entry is idempotent
(not duplicated on repeated installs).

**Editor argument: explicit and required.**
The user must specify the editor (`copilot`, `claude`, or `cursor`). Auto-detection
was considered but rejected — it adds ambiguity when multiple editors are installed
and requires heuristics that can silently install for the wrong target.

**Architecture: hexagonal, consistent with existing commands.**
The `Installer` port (`port.go`) decouples the service from filesystem operations,
keeping the service unit-testable. `EmbedSkillsInstaller` holds the embed FS and
all OS interactions (`homeDir`, `mkdirAll`, `writeFile`, `readFile`), injected via
`NewEmbedSkillsInstallerWithDeps` for testing.

## Consequences
- `idx skills install <editor>` has no external runtime dependencies
- Skills are versioned with the binary; updating skills requires a new `idx` release
- The `--verbose` flag was removed (no subprocess output to stream)
- Installation is instantaneous
