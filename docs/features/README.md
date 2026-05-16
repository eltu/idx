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
| Tools | `skills` |
| Config | `config`, `config show` |

## Recommended usage flow

1. Run `idx init` once per project.
2. Keep indexes fresh with `idx sync`, `idx watch`, or `idx daemon`.
3. Verify index freshness with `idx status`.
4. Use `idx search` during development.
5. Use `idx read <path>` to stream file content — repeated reads boost that file in future search rankings.
6. Run `idx skills install <editor>` once to install AI coding skills into your editor.
7. Use `idx version` (or `idx --version`) when you need build/version info.
