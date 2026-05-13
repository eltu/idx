# ADR 0013: Skills Install Command

## Status
Accepted

## Context
Users need a way to install idx skills into their editors (GitHub Copilot, Claude Code, Cursor)
directly from the CLI. The skills are maintained in the public repository
https://github.com/eltu/idx-skills and distributed as a shell script (`install-skills.sh`)
that accepts an editor argument.

## Decision
Introduce `idx skills install <editor>` as a new command under a new `Tools:` group.

**Fetch strategy: `git clone` over curl-pipe.**
Cloning to a temp directory is preferred over `curl | bash` because:
- The local copy can be inspected before execution
- Git protocol provides integrity guarantees (object hashing)
- No dependency on curl being installed
- Failures are cleaner to diagnose (clone error vs. partial pipe)

The temp directory is removed via `defer` after the install script runs,
ensuring cleanup even when the script fails.

**Editor argument: explicit and required.**
The user must specify the editor (`copilot`, `claude`, or `cursor`). Auto-detection
was considered but rejected — it adds ambiguity when multiple editors are installed
and requires heuristics that can silently install for the wrong target.

**Architecture: hexagonal, consistent with existing commands.**
A `SkillsInstaller` port in `core/ports/` decouples the service from `os/exec`,
keeping the service unit-testable without network or filesystem access.
The `OSSkillsInstaller` adapter in `adapters/repository/` holds all system calls.

## Consequences
- `idx skills install <editor>` requires `git` to be in `$PATH`
- The install script runs with the user's current permissions
- Output from `git clone` and the install script is streamed in real-time to stdout
- Cleanup always runs via defer, even on script failure
