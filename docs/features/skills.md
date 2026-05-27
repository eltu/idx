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
- Copies the bundled `idx-search` skill files from the binary into `~/.<editor>/skills/idx-search/`.
- For `claude` only: reads `~/.claude/settings.json`, adds `"Bash(idx *)"` to `permissions.allow` if not already present, and writes the file back. Creates `~/.claude/settings.json` with a minimal structure if the file does not exist.
- All skill files are written with permission `0600`; skill directories with `0750`.
- If the skill directory already exists, files are overwritten in place (idempotent).

### Installation paths by editor

| Editor | Skills directory |
| --- | --- |
| `claude` | `~/.claude/skills/idx-search/` |
| `cursor` | `~/.cursor/skills/idx-search/` |
| `copilot` | `~/.copilot/skills/idx-search/` |

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

## Examples

```bash
# Install skills for Claude Code
idx skills install claude

# Install skills for GitHub Copilot
idx skills install copilot

# Install skills for Cursor
idx skills install cursor

# Show help with supported editors
idx skills install --help
idx skills
```
