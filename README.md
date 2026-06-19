# idx

Fast BM25 full-text search for Git repositories. Index your codebase once, search it instantly — from the terminal or from your AI coding assistant.

## Install

**Homebrew (macOS / Linux)**

```bash
brew tap eltu/idx
brew install idx
```

**Build from source** (requires Go 1.26+)

```bash
git clone https://github.com/eltu/idx.git
cd idx
make build && cp bin/idx /usr/local/bin/
```

## Getting started

Three commands to go from zero to searching:

```bash
# 1. Build the index for your project
idx init

# 2. Start the background agent (keeps the index up to date automatically)
idx agent start

# 3. Search
idx search "error handling"
```

After `idx init` you can also let the agent handle initialization on its own:

```bash
idx agent start   # starts and auto-indexes if no index exists yet
idx search "your query"
```

### Verify everything is working

```bash
idx agent status  # ✅ Agent running (PID: 12345, uptime: 10s)
idx status        # shows index freshness per directory
```

### Search examples

```bash
# Basic search
idx search "authentication middleware"

# Filter by file extension
idx search -e go "func main"
idx search -e go -e ts "interface"

# Filter by path
idx search -p internal "logger"

# Match any term (OR mode)
idx search --any "init sync destroy"

# AND relaxation — match at least 2 of the 3 terms
idx search --relax 2 "init sync destroy"

# Compact output for AI agents
idx search --compact "BM25"

# JSON output
idx search -j --pretty "config"

# Count matching files only
idx search --count "TODO"
```

### Install AI editor skills

Integrate idx into your editor's AI assistant in one command:

```bash
idx skills install claude    # Claude Code
idx skills install copilot   # GitHub Copilot
idx skills install cursor    # Cursor
```

## Command reference

| Command | Description |
|---|---|
| `idx init` | Build the full index for the project |
| `idx agent start` | Start the background agent |
| `idx agent stop` | Stop the background agent |
| `idx agent status` | Show agent health and PID |
| `idx sync` | Incrementally re-index changed directories |
| `idx status` | Check index freshness |
| `idx search <terms>` | Search indexed content |
| `idx read <path>` | Print a file (boosts it in future search results) |
| `idx inspect [path]` | Browse the index in an interactive TUI |
| `idx config show` | Show resolved project configuration |
| `idx destroy` | Remove all index files and stop the agent |

Full flag reference: [docs/features/README.md](docs/features/README.md)

## Configuration

Create `.idx.yml` at your project root to customize behavior:

```yaml
search:
  operator: AND
  limit: 20
  relaxation: 2

bm25:
  popularity_weight: 0.3  # boost frequently-read files in search results

watch:
  debounce: 750ms

index:
  ignore:
    - vendor/
    - node_modules/
```

See `idx config show` for all available keys and their current values.

## Documentation

| Section | Description |
|---|---|
| [docs/features/](docs/features/README.md) | Full flag reference, organized by command |
| [docs/adr/](docs/adr/README.md) | Architecture Decision Records — design rationale and tradeoffs |
| [docs/releases/](docs/releases/README.md) | Release notes for every stable version |
| [docs/roadmap/](docs/roadmap/README.md) | Planned versions and feature design documents |
| [docs/benchmarks/](docs/benchmarks/README.md) | Performance and token-usage benchmarks |

## Development

```bash
make build      # build binary to bin/idx
make check      # format + lint + tests
make test       # run tests only
make fmt        # run gofmt
make lint       # run golangci-lint
make clean      # remove build artifacts
```
