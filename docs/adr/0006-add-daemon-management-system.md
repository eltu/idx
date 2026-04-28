# ADR 0006: Add Daemon Management System

## Status

Accepted

## Context

The `idx watch` command (from ADR 0005) enables realtime index synchronization, but
requires manual management: users must start the process manually, keep it running
in a terminal session, and manage it across multiple projects.

This creates friction for developer workflows:

- Running `idx watch` in each project directory occupies a terminal tab/pane.
- Processes terminate when shell sessions close unexpectedly.
- No centralized visibility into which projects are being monitored.
- Multiple projects require multiple manual watch instances.

The solution must:

- Enable easy enable/disable of watch monitoring per project.
- Persist state across shell sessions and system restarts.
- Support monitoring multiple projects simultaneously.
- Be testable without requiring real process execution in CI/CD.

## Decision

Introduce a daemon management system with three new commands:

- `idx daemon enable <path>`: Starts a background watch process for a project.
- `idx daemon disable <path>`: Stops the background watch process.
- `idx daemon status`: Lists all monitored projects and their status.

Implementation details:

### State Storage

- State file: `~/.idx/daemon.state` (user home directory).
- Format: JSON with list of monitored projects, timestamps, and PIDs.
- Permissions: `0600` (user-only access) for security.
- Auto-created on first use; subsequent operations read/update atomically.

### Process Management

- Each monitored project runs `idx watch --debounce 750ms` as a detached child process.
- PIDs are stored in state file for lifecycle tracking.
- `daemon enable` auto-initializes projects missing the index (triggers `idx init` if needed).
- `daemon disable` kills the process by PID and removes from state.
- `daemon status` shows all projects with running/stopped status.

### Watch Protection

- `idx watch` command detects if daemon is already monitoring the project.
- Manual `idx watch` execution is rejected with clear error message if daemon active.
- Prevents accidental duplicate watch processes on same project.

### Process Spawning Abstraction

- New interface `ports.ProcessSpawner` encapsulates process creation.
- Real implementation: `OSProcessSpawner` uses `exec.Command()`.
- Test implementation: `fakeProcessSpawner` mocks process spawning.
- Enables unit tests to run without requiring `idx` binary in `$PATH`.
- Critical for CI/CD environments where binary is not yet installed.

### Dependency Injection

- `DaemonService` takes `ProcessSpawner` as constructor parameter.
- Promotes testability and decouples business logic from OS process management.
- Follows ports & adapters pattern.

## Alternatives Considered

### Store state in git metadata (`.git/config`)

- Pros: integrates with git workflow, survives `git clone`.
- Cons: requires git repo, pollutes git config, breaks for non-git projects.

### Store state per project (`.idx/daemon.pid`)

- Pros: no centralized state file, scoped to project.
- Cons: cannot list all monitored projects globally, harder to clean up orphaned PIDs.

### Use systemd user services / launchd agents

- Pros: OS-native process management, automatic restart on reboot.
- Cons: complex setup, platform-specific, breaks portability (Windows).

### Inline process spawning (no interface abstraction)

- Pros: simpler code, fewer files.
- Cons: impossible to test without real `idx` binary, CI/CD failures, mock-heavy test code.

## Consequences

### Positive

- Centralized daemon management for multiple projects via single command.
- Persistent state survives shell restarts and system reboots.
- Protects watch processes from accidental manual execution.
- ProcessSpawner abstraction enables pure unit testing without OS dependencies.
- CI/CD pipelines can run full test suite without installing binary.
- Clear error messages guide users when daemon conflicts with manual watch.
- Minimal performance overhead (single state file I/O per enable/disable/status).

### Negative

- Introduces new state file (`~/.idx/daemon.state`) requiring cleanup on uninstall.
- Long-lived processes require manual `daemon disable` to stop monitoring.
- PIDs stored in state may become stale if processes are killed externally.
- `daemon status` reports "stopped" for externally-killed processes (requires refresh).
- Adds complexity to CLI command routing and testing infrastructure.
- Requires users to learn new daemon commands (not intuitive without documentation).

### Trade-offs

- Chose `~/.idx/daemon.state` (centralized) over `.idx/daemon.pid` (per-project) for:
  - Global `daemon status` visibility across all projects.
  - Simpler orphaned PID cleanup logic.
  - Single point of truth for daemon state.

- Chose `ProcessSpawner` interface over direct `exec.Command()` for:
  - Testability without binary in PATH.
  - No network/filesystem dependencies in unit tests.
  - Faster CI/CD builds (no process spawning overhead).

## Implementation Notes

- Cyclomatic complexity of `Enable()` reduced from 16 to 15 by extracting helper methods:
  - `validateProjectPath()`: path resolution and validation.
  - `checkAlreadyMonitored()`: duplicate check.
  - `ensureIndexExists()`: auto-init logic.
- All daemon operations validate absolute paths to prevent state corruption.
- Tests use `t.TempDir()` for real filesystem isolation (not mocked FS).
- 29 unit and regression tests provide coverage for enable, disable, status, and edge cases.
