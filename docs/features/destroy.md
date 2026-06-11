# destroy

## Purpose

Remove `.idx` metadata recursively from the current project and stop the background agent. Run `idx init` afterwards to rebuild.

## Usage

```bash
idx destroy
```

## Arguments

- None.

## Flags

- Global: `--quiet`, `-q`.

## Behavior and Side Effects

- Sends `idx.destroy` to the running agent via RPC so the agent can clean up in-memory state before the files are removed.
- After the RPC completes, sends `SIGTERM` to stop the background agent. Stop errors indicating the agent was already stopped or state was not found are ignored.
- Resolves current directory and Git root.
- Must run from the project root.
- Recursively traverses directories.
- Skips `.git` directories.
- Removes every `.idx` directory tree found.

## Output

- Success: `🧹 Index metadata removed from project.`

## Errors

- Current directory cannot be resolved.
- Current directory is not project root.
- Directory traversal read errors.
- One or more `.idx` directories could not be removed.

## Examples

```bash
# Remove index and stop agent, then rebuild
idx destroy
idx init
idx agent start
```
