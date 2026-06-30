# skills

## Purpose

Install idx skills bundled in the binary into the current editor's skills directory. No network connection, git binary, or external dependencies are required.

## Usage

```bash
idx skills install <editor>
```

## Arguments

- `install <editor>`: required subcommand. `editor` must be one of: `copilot`, `claude`, `cursor`.

## Flags

| Flag | Type | Default | Notes |
| --- | --- | --- | --- |
| `--quiet`, `-q` | bool | `false` | Suppress informational output |

## Behavior and Side Effects

- Validates the `editor` argument against the supported list.
- Copies the bundled `idx` skill files from the binary into `~/.<editor>/skills/idx/`.
- All skill files are written with permission `0600`; skill directories with `0750`.
- If the skill directory already exists, files are overwritten in place (idempotent).

### Claude-specific behavior

When `editor` is `claude`, the installer also:

1. **Global settings** (`~/.claude/settings.json`):
   - Adds `"Bash(idx *)"` to `permissions.allow` if not already present.
   - Copies four enforcement hook scripts to `~/.claude/`:
     - `idx-block.sh` — blocks Bash shell tools (`grep`, `rg`, `ag`, `cat`, `head`, `tail`, etc.) from reading repository files.
     - `idx-read-block.sh` — blocks Claude's built-in **Read** tool on repository files.
     - `idx-grep-block.sh` — blocks Claude's built-in **Grep** tool on repository files.
     - `idx-context-hook.sh` — injects idx enforcement rules into every session turn.
   - Creates `~/.claude/settings.json` with a minimal structure if the file does not exist. Existing fields are preserved.

2. **Project settings** (`.claude/settings.json` at the git root, when run inside a git repository):
   - Registers three `PreToolUse` hooks:
     - `~/.claude/idx-block.sh` (matcher: `Bash`)
     - `~/.claude/idx-read-block.sh` (matcher: `Read`)
     - `~/.claude/idx-grep-block.sh` (matcher: `Grep`)
   - Registers `~/.claude/idx-context-hook.sh` as a `UserPromptSubmit` hook (no matcher).
   - Hook commands use literal `~` so the file is portable and can be versioned.
   - All operations are idempotent: re-running install never duplicates entries.
   - `CLAUDE.md` is never modified.

### Installation paths by editor

| Editor | Skills directory |
| --- | --- |
| `claude` | `~/.claude/skills/idx/` |
| `cursor` | `~/.cursor/skills/idx/` |
| `copilot` | `~/.copilot/skills/idx/` |

### Hook scripts installed for Claude

| File | Event | Matcher | Purpose |
| --- | --- | --- | --- |
| `~/.claude/idx-block.sh` | `PreToolUse` | `Bash` | Blocks shell tools (grep, cat, head, tail, rg…); redirects to `idx search` / `idx read` |
| `~/.claude/idx-read-block.sh` | `PreToolUse` | `Read` | Blocks Claude's built-in Read tool on repo files |
| `~/.claude/idx-grep-block.sh` | `PreToolUse` | `Grep` | Blocks Claude's built-in Grep tool on repo files |
| `~/.claude/idx-context-hook.sh` | `UserPromptSubmit` | — | Injects idx enforcement rules into each session turn |

## Output

```
  🎯 idx Skills Installer
  Editor: Claude Code

  [1/2]  Installing skills for Claude Code...

  ✓  Skills installed successfully for claude.
     Restart your editor to activate the new skills.
```

## Errors

- Missing editor argument: displays a styled usage panel with the supported editor list. The command exits with an error but Cobra does not print extra usage text.
- Unsupported editor value: `unsupported editor "...": expected one of [copilot claude cursor]`
- Home directory unavailable: `failed to install skills for "...": failed to resolve home directory: ...`
- File or directory write failure: `failed to install skills for "...": ...`
- For `claude`, settings parse failure: `failed to install skills for "claude": failed to parse "...": ...`

## Drift detection

The `make skill-lint` target (also run in CI) validates that every flag and subcommand
referenced in the skill asset files exists in the actual CLI binary. Any drift between
skill docs and the live CLI fails the build immediately.

## Examples

```bash
# Install skills for Claude Code (also configures project hooks if inside a git repo)
idx skills install claude

# Install skills for GitHub Copilot
idx skills install copilot

# Install skills for Cursor
idx skills install cursor

# Show help with supported editors
idx skills install --help
idx skills
```
