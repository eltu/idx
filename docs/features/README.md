# idx Features

Detailed end-user command reference, organized by command.

## Navigation

- [init](init.md)
- [sync](sync.md)
- [status](status.md)
- [search](search.md)
- [read](read.md)
- [inspect](inspect.md)
- [watch](watch.md)
- [destroy](destroy.md)
- [version](version.md)
- [skills](skills.md)
- [server](server.md)
- [config](config.md)
- [errors](errors.md)
- [daemon](daemon.md) *(removed in v0.5.0 — see daemon.md for migration)*

## Global flags

All commands inherit the root global flags:

- `--quiet`, `-q`: suppress informational output (errors are still written to stderr).

## Command groups

| Group | Commands |
| --- | --- |
| Index Setup | `init`, `destroy` |
| Index Sync | `sync`, `watch`, `status` |
| Search | `search`, `inspect`, `read` |
| About | `version` |
| Tools | `skills`, `server` |
| Config | `config`, `config show` |

## Recommended usage flow

1. Run `idx init` once per project to build the initial index.
2. Run `idx server start` to start the daemon — all search/index commands require the server.
3. Keep indexes fresh automatically — the server runs the watch loop internally.
4. Verify index freshness with `idx status`.
5. Use `idx search` during development.
6. Use `idx read <path>` to stream file content — repeated reads boost that file in search rankings via the read-popularity signal.
7. Run `idx skills install <editor>` once to install AI coding skills into your editor.
8. Use `idx version` (or `idx --version`) when you need build/version info.
