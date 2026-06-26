# ADR 0023: Skills Project-Level Enforcement and Skill Asset Drift Detection

## Status
Accepted

## Context

Two independent problems emerged as the `idx skills install` command matured:

**Problem 1 — Cognitive vs. binding enforcement.**
The `SKILL.md` file instructs Claude to use `idx search` and `idx read` instead of
`grep`/`cat`/`head`/`tail`, but this is only a cognitive signal. Under long contexts or
when competing instructions are present, Claude may silently fall back to shell tools.
No mechanism existed to intercept and block those calls at runtime.

**Problem 2 — Skill asset drift.**
`SKILL.md` and `references/idx-commands.md` contain flag and subcommand names
hardcoded as prose. As the CLI evolves, these files can reference extinct flags
(e.g. `--agent-compact` after it was renamed to `--compact`) or removed commands
(e.g. `idx watch`, `idx daemon enable/disable`) with no automated warning.

## Decisions

### 1. Project-level enforcement via PreToolCall + UserPromptSubmit hooks

When `idx skills install claude` is run inside a project, the project's
`.claude/settings.json` is updated with two hook registrations. No other project files
are modified — in particular, `CLAUDE.md` is intentionally left untouched.

**Why not CLAUDE.md?**
An earlier iteration injected a sentinel-guarded section into `CLAUDE.md`. This was
discarded because it permanently modifies a file typically tracked in version control,
creating noise for team members who do not use `idx`. A hook-based approach achieves
the same effect without touching any project-owned file.

**PreToolCall hook (`assets/idx-search/claude-project/block-shell-tools.sh`).**
Copied to `~/.claude/idx-search-block.sh` on install. Registered in the project's
`.claude/settings.json` with a `"matcher": "Bash"` filter. The hook reads the Bash
`command` from stdin (JSON-RPC input), inspects the first pipeline segment, and exits 2
(block + feedback to Claude) if the first word is one of `grep`, `egrep`, `fgrep`,
`rg`, `ag`, `ack`, `pt`, `ugrep`, `git grep`, `cat`, `head`, or `tail`. Exit 0 passes
through. `jq` is used when available; falls back to POSIX `grep`+`sed`.

**UserPromptSubmit hook (`assets/idx-search/claude-project/context-hook.sh`).**
Copied to `~/.claude/idx-search-context-hook.sh` on install. Registered in the
project's `.claude/settings.json` under `UserPromptSubmit` (no matcher). Claude Code
runs this hook before processing each user turn and injects its stdout as additional
context. The script emits the `NEVER/ALWAYS` enforcement rules every turn, making them
resistant to context-window eviction.

```json
{
  "hooks": {
    "PreToolCall": [
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "~/.claude/idx-search-block.sh"}]}
    ],
    "UserPromptSubmit": [
      {"hooks": [{"type": "command", "command": "~/.claude/idx-search-context-hook.sh"}]}
    ]
  }
}
```

**Scope and idempotency.**
Both hook registrations are project-scoped (not global). Existing fields in
`.claude/settings.json` are preserved. Running `idx skills install claude` twice in the
same project produces exactly one entry per event type in `settings.json`.

**`projectRoot == ""`** skips project configuration entirely, preserving backward
compatibility for callers that have no git root (e.g. the standalone server mode).

### 2. Skill asset drift detection via `make skill-lint`

A shell script (`scripts/validate-skill-assets.sh`) compares skill Markdown files
against the live CLI binary in three phases:

1. **Subcommand presence** — verifies that every subcommand named in the asset files
   (e.g. `idx search`, `idx read`, `idx server`) is present in `idx --help`.
2. **Search flags** — extracts backtick-fenced `--flag` tokens from the
   "Search Flags Reference" section and checks each against `idx search --help`.
3. **Read flags** — same check for the "Read Flags Reference" section against
   `idx read --help`.

Any missing flag or subcommand causes exit 1 with a descriptive message. The script
runs as part of CI via a `skill-lint` Makefile target that depends on `build`, ensuring
the binary is always fresh before validation.

## Consequences

- Claude operating in a project with `idx` installed cannot use `grep`/`cat`/`tail`
  on repository files: the PreToolCall hook blocks at the Bash layer, and the
  UserPromptSubmit hook re-injects the enforcement rules on every turn so they survive
  long contexts.
- `CLAUDE.md` is never modified; the enforcement is entirely self-contained in
  `.claude/settings.json` and the two scripts in `~/.claude/`.
- Project-level `.claude/settings.json` changes are checked in with the project,
  making the enforcement visible and reviewable by the team.
- The `claude-project/` subdirectory of the embedded assets is excluded from the skill
  file copy; its scripts are installed directly to `~/.claude/` by the installer.
- Skill asset files must stay in sync with the CLI; any flag or subcommand rename that
  is not reflected in the assets will fail CI immediately.
- The `Installer` port signature changed from `Install(editor string)` to
  `Install(editor, projectRoot string)`, propagating through the service, mock, and CLI
  adapter. Empty `projectRoot` preserves existing behaviour.
- The hook scripts have no runtime dependency on `idx` itself — they only inspect
  command strings or emit static text, so they work before the daemon is started.
