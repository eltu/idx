# idx

Fast code and text search CLI for Git repositories, powered by BM25 and per-directory indexes.

⚠️ This project is under active development and may contain bugs or breaking changes.

## Requirements

- Go `1.26+`
- Git repository (project root is resolved from `.git`)

## Quick start

```bash
make build
./bin/idx init
./bin/idx search "auth token"
```

Or run directly:

```bash
go run cmd/idx/main.go <command>
```

## Core commands

- `idx init`: create indexes recursively
- `idx sync`: resync existing indexes
- `idx search <terms>`: search indexed content
- `idx watch`: realtime sync in active terminal session
- `idx daemon enable|disable|status`: background monitoring
- `idx inspect <path>`: inspect index content
- `idx destroy`: remove index metadata

## Useful Make targets

- `make build`
- `make test`
- `make check`
- `make fmt`
- `make lint`
- `make complexity`
- `make clean`

## Detailed docs

For full command reference, flags, examples, and troubleshooting, see:

- [docs/features/README.md](docs/features/README.md)

## Additional docs

- Benchmarks: [docs/benchmarks](docs/benchmarks)
- Architecture decisions: [docs/adr](docs/adr)
