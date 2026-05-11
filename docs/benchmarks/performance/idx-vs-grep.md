# idx vs grep Benchmark Report

Date: 2026-05-06  
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
| files-500 | service | 28,625 | 0.029 | 15,166 | 486 |
| files-500 | cli | 8,379,099 | 8.38 | 14,555 | 43 |
| files-500 | grep | 14,750,834 | 14.75 | 18,229 | 80 |
| files-2000 | service | 115,375 | 0.115 | 60,562 | 1,910 |
| files-2000 | cli | 18,603,417 | 18.60 | 14,784 | 43 |
| files-2000 | grep | 55,202,165 | 55.20 | 18,796 | 80 |

## Key comparisons

### idx CLI vs grep

- `files-500`: `idx` is approximately `1.76x` faster than `grep`
- `files-2000`: `idx` is approximately `2.97x` faster than `grep`

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
