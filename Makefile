BINARY     := idx
BUILD_DIR  := bin
CMD_MAIN   := ./cmd/idx
ALL_PKGS   := ./...

.PHONY: all build fmt lint test clean check

## Default: format, lint, test, and build
all: fmt lint test build

## Build the main binary
build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY) $(CMD_MAIN)

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

## Format + lint + test (no build)
check: fmt lint test

## Remove build artifacts
clean:
	rm -rf $(BUILD_DIR)
