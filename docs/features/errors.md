# Common Errors

## Input and flag validation

- `missing search query: ... expected idx search <terms>`
- `invalid --debounce value ... expected a duration greater than 0`
- `--json-pretty requires --format json`
- `unsupported --format value ... expected one of [text json]`
- `invalid --context value ... expected a non-negative integer`
- `invalid --from value ... expected a non-negative integer`
- `invalid --size value ... expected a positive integer`

## State and environment

- Missing index before `search`/`sync`.
- Running commands outside expected Git root.
- File permission/read/write errors.

## Recovery quick guide

1. Run `idx init` if indexes are missing.
2. Re-run from Git project root when required.
3. Validate flags and argument counts.
4. If using background mode, check `idx daemon status`.
