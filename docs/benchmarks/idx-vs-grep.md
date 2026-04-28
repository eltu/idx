# idx vs grep Benchmark Report

Date: 2026-04-27  
Repository: idx (self-benchmark in this repo)

## Scope

This report compares search performance between:

- `idx` service (in-process search service)
- `idx` CLI (`idx search needle`)
- `grep` (`grep -Rnw --exclude-dir=.git --exclude-dir=.idx needle .`)

Benchmark command used:

```bash
go test ./internal/core/services/search -run '^$' -bench '^BenchmarkSearchVsGrep$' -benchmem -count=3
```

## Corpus profiles

- `files-500`: 20 directories x 25 files (500 files)
- `files-2000`: 40 directories x 50 files (2000 files)

## Results (average of 3 runs)

| Corpus | Method | ns/op (avg) | ms/op | B/op | allocs/op |
|---|---:|---:|---:|---:|---:|
| files-500 | service | 30,795 | 0.031 | 17,150 | 569 |
| files-500 | cli | 5,711,457 | 5.71 | 11,747 | 43 |
| files-500 | grep | 13,790,078 | 13.79 | 15,987 | 88 |
| files-2000 | service | 123,636 | 0.124 | 68,965 | 2,260 |
| files-2000 | cli | 13,894,524 | 13.89 | 11,944 | 43 |
| files-2000 | grep | 51,318,395 | 51.32 | 16,624 | 88 |

## Key comparisons

### idx CLI vs grep

- `files-500`: `idx` is approximately `2.41x` faster than `grep`
- `files-2000`: `idx` is approximately `3.69x` faster than `grep`

### Allocations and memory

- `idx` CLI uses fewer allocations than `grep` (`43` vs `88` allocs/op)
- `idx` CLI also uses fewer bytes per operation in these runs

## Interpretation

- `grep` scans raw files on each run.
- `idx` searches prebuilt indexes, so query-time work is lower.
- The in-process service is much faster than CLI because it avoids process-spawn overhead.
- The relative advantage of `idx` over `grep` increases with larger corpus size in this benchmark.

## Notes and limitations

- This benchmark focuses on query latency, not index creation cost (`init`/`sync`).
- CLI results include process startup cost.
- Absolute numbers may vary by machine, filesystem, and system load.
- Use this benchmark for directional comparison under the same environment.

## Reproduce

```bash
make bench-search-vs-grep
# or
go test ./internal/core/services/search -run '^$' -bench '^BenchmarkSearchVsGrep$' -benchmem -count=3
```
