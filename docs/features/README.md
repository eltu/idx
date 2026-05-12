# idx Features

Detailed end-user command reference, organized by command.

## Navigation

- [init](init.md)
- [sync](sync.md)
- [status](status.md)
- [search](search.md)
- [inspect](inspect.md)
- [watch](watch.md)
- [daemon](daemon.md)
- [destroy](destroy.md)
- [version](version.md)
- [errors](errors.md)

## Global flags

All commands inherit the root global flags:

- `--quiet`, `-q`: suppress informational output (errors are still written to stderr).

## Recommended usage flow

1. Run `idx init` once.
2. Keep indexes fresh with `idx sync`, `idx watch`, or `idx daemon`.
3. Verify index freshness with `idx status`.
4. Use `idx search` during development.
5. Use `idx version` (or `idx --version`) when you need build/version info.
