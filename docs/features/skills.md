# skills

## Purpose

Install idx skills from the public repository [github.com/eltu/idx-skills](https://github.com/eltu/idx-skills) into the current editor.

## Usage

```bash
idx skills install <editor> [flags]
```

## Arguments

- `install <editor>`: required subcommand. `editor` must be one of: `copilot`, `claude`, `cursor`.

## Flags

| Flag | Type | Default | Notes |
| --- | --- | --- | --- |
| `--verbose` | bool | `false` | Stream git clone and installer output to stdout |
| `--quiet`, `-q` | bool | `false` | Suppress informational step output |

## Behavior and Side Effects

- Clones `https://github.com/eltu/idx-skills` into a temporary directory.
- Executes `./install-skills.sh <editor>` inside the cloned repository.
- Removes the temporary directory after the script finishes (even on failure).
- Requires `git` to be installed and available in `$PATH`.
- Without `--verbose`, git clone and installer output are suppressed; only step markers and the final result are shown.
- With `--verbose`, all subprocess output (git progress, installer messages) is streamed in real-time.

## Output

Step markers are always shown:

```
  🎯 idx Skills Installer
  Editor: Claude Code

  [1/3]  Cloning github.com/eltu/idx-skills...
  [2/3]  Installing skills for claude...
  [3/3]  Cleaning up...

  ✓  Skills installed successfully for claude.
     Restart your editor to activate the new skills.
```

With `--verbose`, subprocess output appears inline after each step marker.

## Errors

- Missing editor argument: displays usage with supported editor list — no exit 0 (styled inline error, not returned to cobra).
- Unsupported editor value: `unsupported editor "...": expected one of [copilot claude cursor]`
- Clone failure: `failed to clone idx-skills: git clone failed: exit status ...`
- Script failure: `install script failed for "...": install-skills.sh exited with error: exit status ...`
- Temporary directory creation failure: `failed to create temp directory: ...`

## Examples

```bash
# Install skills for Claude Code (silent mode)
idx skills install claude

# Install skills for GitHub Copilot with full output
idx skills install copilot --verbose

# Install skills for Cursor
idx skills install cursor

# Show help with supported editors
idx skills install --help
idx skills
```
