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
- [errors](errors.md)

## Recommended usage flow

1. Run `idx init` once.
2. Keep indexes fresh with `idx sync`, `idx watch`, or `idx daemon`.
3. Verify index freshness with `idx status`.
4. Use `idx search` during development.
5. For exploratory multi-term queries, start with `--operator OR` or use `--operator AND --relaxation '>N'` when you want strict ranking with controlled fallback.
