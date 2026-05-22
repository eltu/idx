BINARY     := idx
BUILD_DIR  := bin
CMD_MAIN   := ./cmd/idx
ALL_PKGS   := ./...
CYCLO_LIMIT := 15
VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS    := -X main.version=$(VERSION) -X main.buildDate=$(BUILD_DATE)

.PHONY: all build fmt lint test complexity clean check bench-sync bench-search-vs-grep test-concurrency test-concurrency-race test-concurrency-heavy test-concurrency-ci

## Default: format, lint, test, and build
all: fmt lint test build

## Build the main binary
build:
	@mkdir -p $(BUILD_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) $(CMD_MAIN)

## Apply gofmt to all Go source files
fmt:
	gofmt -w $$(find . -name '*.go' -not -path './.git/*')

GOLANGCI   := $(shell go env GOPATH)/bin/golangci-lint

## Run golangci-lint (configured via .golangci.yml)
lint:
	$(GOLANGCI) run $(ALL_PKGS)

## Run unit tests
test:
	go test $(ALL_PKGS)

## Run unit tests with coverage report (used by SonarCloud)
coverage:
	go test $(ALL_PKGS) -coverprofile=coverage.out -covermode=atomic

## Fail when any function exceeds cyclomatic complexity threshold
complexity:
	@go run github.com/fzipp/gocyclo/cmd/gocyclo@latest . | sort -rn | awk -v limit=$(CYCLO_LIMIT) '$$0 !~ /_test\.go:/ && $$1 > limit {print; found=1} END {if (found) exit 1}'

## Benchmark sync incremental mode (time + file read syscalls proxy)
bench-sync:
	go test ./internal/core/services/indexing -run '^$$' -bench '^BenchmarkSyncMetadataIncremental$$' -benchmem

## Benchmark idx search vs grep
bench-search-vs-grep:
	go test ./internal/core/services/search -run '^$$' -bench '^BenchmarkSearchVsGrep$$' -benchmem

## Concurrency test: sync writes while search reads
test-concurrency:
	IDX_RUN_CONCURRENCY_TESTS=1 go test ./internal/core/services/search -run '^TestSyncAndSearchRunConcurrently' -count=1

## Concurrency test with race detector (recommended for CI)
test-concurrency-race:
	IDX_RUN_CONCURRENCY_TESTS=1 go test ./internal/core/services/search -run '^TestSyncAndSearchRunConcurrently' -race -count=$${RACE_COUNT:-3}

## Concurrency test profile used by CI workflow
test-concurrency-ci:
	RACE_COUNT=$${RACE_COUNT:-2} \
	IDX_CONCURRENCY_FILES=$${IDX_CONCURRENCY_FILES:-60} \
	IDX_CONCURRENCY_SUBDIRS=$${IDX_CONCURRENCY_SUBDIRS:-4} \
	IDX_CONCURRENCY_FILES_PER_DIR=$${IDX_CONCURRENCY_FILES_PER_DIR:-16} \
	IDX_CONCURRENCY_SYNC_ITERATIONS=$${IDX_CONCURRENCY_SYNC_ITERATIONS:-70} \
	IDX_CONCURRENCY_SEARCH_WORKERS=$${IDX_CONCURRENCY_SEARCH_WORKERS:-4} \
	IDX_CONCURRENCY_SEARCH_ITERATIONS=$${IDX_CONCURRENCY_SEARCH_ITERATIONS:-110} \
	IDX_CONCURRENCY_TIMEOUT_SECONDS=$${IDX_CONCURRENCY_TIMEOUT_SECONDS:-45} \
	$(MAKE) test-concurrency-race

## High-load concurrency test (configurable via env vars)
test-concurrency-heavy:
	IDX_RUN_CONCURRENCY_TESTS=1 \
	IDX_CONCURRENCY_FILES=$${IDX_CONCURRENCY_FILES:-240} \
	IDX_CONCURRENCY_SUBDIRS=$${IDX_CONCURRENCY_SUBDIRS:-10} \
	IDX_CONCURRENCY_FILES_PER_DIR=$${IDX_CONCURRENCY_FILES_PER_DIR:-50} \
	IDX_CONCURRENCY_SYNC_ITERATIONS=$${IDX_CONCURRENCY_SYNC_ITERATIONS:-280} \
	IDX_CONCURRENCY_SEARCH_WORKERS=$${IDX_CONCURRENCY_SEARCH_WORKERS:-10} \
	IDX_CONCURRENCY_SEARCH_ITERATIONS=$${IDX_CONCURRENCY_SEARCH_ITERATIONS:-500} \
	IDX_CONCURRENCY_TIMEOUT_SECONDS=$${IDX_CONCURRENCY_TIMEOUT_SECONDS:-60} \
	go test ./internal/core/services/search -run '^TestSyncAndSearchRunConcurrently' -count=1

## Format + lint + test (no build)
check: fmt lint test

## Remove build artifacts
clean:
	rm -rf $(BUILD_DIR)
