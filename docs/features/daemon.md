# daemon

## Purpose

Manage background watch processes for one or more projects.

## Usage

```bash
idx daemon <subcommand>
idx daemon enable <path>
idx daemon disable <path>
idx daemon status
```

## Arguments

- `enable <path>`: requires exactly one path argument.
- `disable <path>`: requires exactly one path argument.
- `status`: no positional arguments.

## Flags

- Global: `--quiet`, `-q`.

## Behavior and Side Effects

- `enable`:
	- Resolves project path to absolute path and validates it exists.
	- Checks daemon state to avoid duplicate monitoring.
	- If already monitored with a live process, returns success without extra output (idempotent).
	- If index is missing, auto-runs init from target path.
	- Spawns background watch process and stores PID/state.
- `disable`:
	- Resolves absolute path.
	- Stops tracked process by PID when available.
	- Removes project from daemon state.
- `status`:
	- Prints monitored project list with running/stopped indication.

## Output

- `enable` success:
	- `✅ Watch enabled for "<abs-path>"`
	- `👀 Monitoring in realtime (PID: <pid>)`
- `enable` when index is missing:
	- `ℹ️  Index not found. Creating initial index...`
- `disable` success:
	- `✅ Watch disabled for "<abs-path>"`
- `status` with no projects:
	- `❌ No projects being monitored`
- `status` with projects:
	- Header: `📊 Monitored Projects:`
	- One line per project with status, path, PID, start time.

## Errors

- CLI argument contract:
	- `expected project path argument` (wrong arg count for enable/disable)
- `enable` path validation failures.
- Failure to start watch process.
- Failure to persist daemon state.
- `disable` when daemon state is empty or project not registered.

## Examples

```bash
idx daemon enable .
idx daemon status
idx daemon disable .
```
