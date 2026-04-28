# daemon

## Contract

- Command group: `idx daemon`
- Purpose: Manage persistent/background monitoring per project.
- Scope: Projects registered in daemon state.

## Subcommands

### `idx daemon enable <path>`

- Enables background monitoring for `<path>`.
- Auto-runs initialization when index is missing.

Parameters:

- `<path>` required

### `idx daemon disable <path>`

- Stops monitoring and removes project from daemon state.

Parameters:

- `<path>` required

### `idx daemon status`

- Lists monitored projects and runtime state.

Parameters:

- No args

## Preconditions

- `enable`/`disable` require exactly one project path argument.
- Project path must exist for `enable`.

## Behavior

- Stores daemon state per user.
- Prevents duplicate monitoring of same project.
- Blocks manual `idx watch` when daemon already monitors project.

## Examples

```bash
idx daemon enable .
idx daemon status
idx daemon disable .
```

## Common failures

- Missing project path argument.
- Already monitored project.
- Invalid project path.
