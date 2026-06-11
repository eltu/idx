# Common Errors

## Agent not running

All commands that route through the agent (`search`, `init`, `sync`, `status`, `read`, `inspect`, `destroy`) require the background agent to be running. If the socket is absent or refuses the connection, the command fails immediately with a styled message:

```
✗ idx agent is not running
  start with: idx agent start
  then retry your command
```

**Recovery:** run `idx agent start` to start the background agent, then retry.

## Command dispatch and argument contract

- `inspect accepts at most one path: got ... expected idx inspect [path]`
- `invalid inspect path "...": expected idx inspect [path]`

## Search flag validation

- `missing search query: got ... expected idx search <terms>`
- `unsupported --format value "...": expected one of [text json]`
- `--pretty requires --format json (or -j): got format "..."`
- `invalid --context value ...: expected a non-negative integer`
- `invalid --skip value ...: expected a non-negative integer`
- `invalid --from value ...: expected a non-negative integer` (deprecated flag)
- `invalid --limit value ...: expected a positive integer`
- `invalid --size value ...: expected a positive integer` (deprecated flag)
- `unsupported --operator value "...": expected one of [AND OR]`
- `invalid search.relaxation with --operator "...": expected "AND"`

## Config flag validation

- `unknown config key "<key>" — valid keys: search.format, search.limit, ...` (from `idx config get`)

## Skills flag validation

- Missing editor (inline styled error, not cobra error):
  - `⚠  Missing editor argument` with usage and editor list
- Unsupported editor value: `unsupported editor "...": expected one of [copilot claude cursor]`

## Index lifecycle and state errors

- `sync must run from project root: got current directory "...", expected root directory "..."`
- `sync requires project root to be indexed: no index found at "...", run idx init first`
- `destroy must run from project root: got current directory "...", expected root directory "..."`
- `no index found at "...": run idx init first` (inspect with path)
- `no index found under project root "...": run idx init first` (inspect without path or status)

## Status-specific validation

- `no index found under project root "...": run idx init first`
- `unindexed directories found` — preceded by a styled warning panel listing the unindexed directories
- `stale index` — preceded by the status overview panel showing `❌ N directory/ies stale — run idx sync`

## Agent lifecycle errors

Not inside an idx project (no `.idx` directory found):

```
✗ Not inside an idx project
  cd <project-root>
  then: idx agent start
```

Agent process failed to start or become ready within the readiness timeout:

- `failed to start server for "...": got error ..., expected process to start`
- `server process (PID: ...) did not become ready within ...`

## Skills install errors

- `failed to install skills for "...": failed to resolve home directory: ...`
- `failed to install skills for "...": failed to read embedded file "...": ...`
- `failed to install skills for "claude": failed to read "...": ...` (settings.json unreadable)
- `failed to install skills for "claude": failed to parse "...": ...` (settings.json malformed JSON)
- `failed to install skills for "claude": failed to write "...": ...` (settings.json write failure)

## Recovery Quick Guide

1. Run `idx agent start` before running `search`, `init`, `sync`, `status`, `read`, `inspect`, or `destroy`.
2. Run `idx init` to bootstrap indexes on first use.
3. Run root-scoped commands (`sync`, `destroy`) from the project root.
4. Validate flags and positional arguments.
5. Use `idx status` to verify whether index logs still match file modification timestamps.
