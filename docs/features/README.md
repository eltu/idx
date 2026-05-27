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
- [daemon](daemon.md)
- [destroy](destroy.md)
- [version](version.md)
- [skills](skills.md)
- [server](server.md)
- [config](config.md)
- [errors](errors.md)

## Global flags

All commands inherit the root global flags:

- `--quiet`, `-q`: suppress informational output (errors are still written to stderr).

## Command groups

| Group | Commands |
| --- | --- |
| Index Setup | `init`, `destroy` |
| Index Sync | `sync`, `watch`, `daemon`, `status` |
| Search | `search`, `inspect`, `read` |
| About | `version` |
| Tools | `skills`, `server` |
| Config | `config`, `config show` |

## Recommended usage flow

1. Start `idx server` (or `idx daemon enable .`) — all search/index commands require the server.
2. Run `idx init` once per project to build the initial index.
3. Keep indexes fresh with `idx sync`, `idx watch`, or `idx daemon`.
4. Verify index freshness with `idx status`.
5. Use `idx search` during development.
6. Use `idx read <path>` to stream file content — repeated reads boost that file in search rankings via the read-popularity signal.
7. Run `idx skills install <editor>` once to install AI coding skills into your editor.
8. Use `idx version` (or `idx --version`) when you need build/version info.
