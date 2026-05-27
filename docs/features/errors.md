# Common Errors

## Server not running

All commands that route through the server (`search`, `init`, `sync`, `status`, `read`) require `idx server` to be running. If the socket is absent or refuses the connection, the command fails immediately with a styled message:

```
✗ idx server not running
  start with: idx server
  or:          idx daemon enable .
```

**Recovery:** run `idx server` in a background terminal, or enable the daemon with `idx daemon enable .`.

## Command dispatch and argument contract

- `inspect accepts at most one path: got ... expected idx inspect [path]`
- `invalid inspect path "...": expected idx inspect [path]`
- `expected project path argument` (daemon enable/disable)

## Search flag validation

- `missing search query: got ... expected idx search <terms>`
- `unsupported --format value "...": expected one of [text json]`
- `--json-pretty requires --format json: got format "..."`
- `invalid --context value ...: expected a non-negative integer`
- `invalid --from value ...: expected a non-negative integer`
- `invalid --size value ...: expected a positive integer`
- `unsupported --operator value "...": expected one of [AND OR]`
- `invalid --relaxation value "...": expected format >N where N is a non-negative integer`
- `invalid --relaxation with --operator "...": expected "AND"`

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

## Watch and daemon errors

- `invalid --debounce value ...: expected a duration greater than 0`
- `failed to run watch command: got invalid debounce ..., expected duration greater than 0`
- `cannot run watch: daemon is already monitoring this project. Disable the daemon with 'idx daemon disable' first`
- `project "..." not being monitored`
- `project "..." not being monitored: no projects active`
- `failed to start watch for "...": got error ..., expected process to start`

## Skills install errors

- `failed to install skills for "...": failed to resolve home directory: ...`
- `failed to install skills for "...": failed to read embedded file "...": ...`
- `failed to install skills for "claude": failed to read "...": ...` (settings.json unreadable)
- `failed to install skills for "claude": failed to parse "...": ...` (settings.json malformed JSON)
- `failed to install skills for "claude": failed to write "...": ...` (settings.json write failure)

## Recovery Quick Guide

1. Start `idx server` (or `idx daemon enable .`) before running `search`, `init`, `sync`, `status`, or `read`.
2. Run `idx init` to bootstrap indexes on first use.
3. Run root-scoped commands (`sync`, `destroy`) from the project root.
4. Validate flags and positional arguments.
5. Use `idx daemon status` before starting `idx watch`.
6. Use `idx status` to verify whether index logs still match file modification timestamps.
