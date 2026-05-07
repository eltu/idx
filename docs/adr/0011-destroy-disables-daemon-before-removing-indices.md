# ADR 0011: `idx destroy` Disables Daemon Before Removing Indices

## Status

Accepted

## Context

`idx destroy` removes all `.idx` directories under the project root. With the
watch-mode daemon introduced in ADR 0005 and ADR 0006, a running watcher
process monitors the filesystem and reacts to file-change events by writing
new `.idx` directories.

When `idx destroy` was run while a daemon was active, the following race
condition occurred:

1. `destroy` scans and deletes `.idx` directories.
2. The still-running daemon detects a file-change event (or a directory
   removal event) and immediately re-creates one or more `.idx` directories.
3. The project appears to still have index files even after `destroy` completes.

This made `idx destroy` appear broken: users found leftover `.idx` directories
even after a seemingly successful destroy.

A second, related bug compounded the problem: `DaemonService.Disable` only
removed the **first** matching entry for a project path from the daemon state
file. If the same path had been registered more than once (e.g. after repeated
`idx daemon enable .` calls), one or more watcher processes remained alive
after disable. The daemon state was therefore not accurately representing
reality, and subsequent operations (status, disable, destroy) could behave
unexpectedly.

## Decision

### 1. Disable daemon before destroy

`cobra_commands.go` wraps the destroy execution with a preflight daemon-disable
step:

```go
func (runner CommandRunner) newDestroyCommand() *cobra.Command {
    return &cobra.Command{
        Use:   "destroy",
        Short: "Destroy index metadata",
        RunE: func(_ *cobra.Command, _ []string) error {
            if err := runner.disableDaemonForDestroy(); err != nil {
                return err
            }
            return runner.destroyCommand.Run()
        },
    }
}
```

The helper `disableDaemonForDestroy` calls `DaemonService.Disable(".")` and
silently ignores the following expected non-error conditions:

- `"daemon not initialized"` — no daemon state file exists; nothing to disable.
- `"not being monitored"` — the project is not in the state file.
- `"no projects active"` — the state file is empty.

Any other error (e.g. permission failure, unexpected state corruption) is
propagated and aborts the destroy.

### 2. Remove all duplicate state entries in `DaemonService.Disable`

`DaemonService.Disable` was updated to iterate the full project list and
remove **every** entry whose path matches `absPath`, rather than stopping
after the first match. For each removed entry, if the associated process is
still running (`Enabled && PID > 0`), the process is killed.

```go
for _, project := range state.Projects {
    if project.Path != absPath {
        filtered = append(filtered, project)
        continue
    }
    removedCount++
    if project.Enabled && project.PID > 0 {
        proc, findErr := os.FindProcess(project.PID)
        if findErr == nil {
            _ = proc.Kill()
        }
    }
}
if removedCount == 0 {
    return fmt.Errorf("project %q not being monitored", absPath)
}
```

## Decision Drivers

- **Correctness over speed**: a destroy that leaves files behind is worse than
  one that takes a few extra milliseconds to stop a watcher.
- **Idempotence**: the destroy command should be safe to run in any daemon
  state (enabled, disabled, or never initialised).
- **State integrity**: duplicate daemon entries are a latent source of bugs
  beyond the destroy race; fixing `Disable` to be exhaustive improves the
  overall reliability of daemon management.

## Consequences

### Positive

- `idx destroy` reliably leaves zero `.idx` directories when completed
  successfully.
- Repeated `idx daemon enable .` calls no longer leave orphaned watcher
  processes after `disable` or `destroy`.
- `DaemonService.Disable` is now idempotent with respect to duplicate entries.

### Negative

- `idx destroy` now has a side effect on daemon state. Users who expected
  destroy to be a pure filesystem operation may be surprised that it also
  terminates background processes.
- If the daemon disable step fails with an unexpected error, the `.idx`
  directories are not removed (fail-fast). Users in that situation must first
  run `idx daemon disable .` manually before retrying destroy.

## Operational Notes

- The `disableDaemonForDestroy` helper and `isIgnorableDestroyDaemonDisableError`
  predicate live in `internal/adapters/handlers/cli/cobra_commands.go`.
- `DaemonService.Disable` is tested for the duplicate-removal behaviour in
  `internal/core/services/daemon/daemon_service_test.go`
  (`TestDaemonServiceDisableRemovesDuplicateProjectEntries`).
- The destroy command itself must still be run from the project root; this
  constraint is unchanged from the original design.
