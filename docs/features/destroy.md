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
- The agent stops its own file-watch loop **before** removing anything. This
  matters because removing a directory's `.idx` produces a filesystem event;
  without the watch loop stopped first, the agent would resync — and thereby
  recreate — the very directory whose index was just deleted.
- Resolves current directory and Git root.
- Must run from the project root.
- Recursively traverses directories.
- Skips `.git` directories.
- Removes every `.idx` directory tree found.
- On success, the agent closes its own listener and the background process
  exits on its own — its socket and state files live inside `.idx/` and are
  gone once destroy completes, so the agent can no longer be reached to stop
  it externally. As a best-effort fallback, the CLI also sends `SIGTERM`
  after the RPC; this is a no-op once the agent has already exited. Stop
  errors indicating the agent was already stopped or state was not found are
  ignored.

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
