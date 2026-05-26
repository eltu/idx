# idx

Fast code and text search CLI for Git repositories, powered by BM25 and per-directory indexes.

⚠️ This project is under active development and may contain bugs or breaking changes.

## Install

### Homebrew (macOS / Linux)

```bash
brew tap eltu/idx
brew install idx
```

Upgrade:

```bash
brew upgrade idx
```

### Build from source

Requires Go `1.26+`.

```bash
git clone https://github.com/eltu/idx.git
cd idx
make build
cp bin/idx /usr/local/bin/
```

Or run without installing:

```bash
go run cmd/idx/main.go <command>
```

## Requirements

- Git repository (project root is resolved from `.git`)
- Go `1.26+` only required when building from source

## Quick start

```bash
idx init
idx search "auth token"
idx search "func abc x y int 10" --operator AND --relaxation '>2'
```

## Core commands

- `idx init`: create indexes recursively
- `idx sync`: resync existing indexes
- `idx search <terms>`: search indexed content, with `--operator AND|OR` and AND relaxation via `--relaxation '>N'`
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
