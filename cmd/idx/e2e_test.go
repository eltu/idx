package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appserver "idx/internal/app/server"
	featdaemon "idx/internal/features/daemon"
	featindexing "idx/internal/features/indexing"
	idxstorage "idx/internal/features/indexing/storage"
	featread "idx/internal/features/read"
	featsearch "idx/internal/features/search"
	sharedconfig "idx/internal/shared/config"
	sharedfs "idx/internal/shared/filesystem"
	idxipc "idx/internal/shared/ipc"
	sharedoutput "idx/internal/shared/output"
)

// newTestProject creates a temp directory with a .git marker and five Go source files
// across three directories. Unique terms per file:
//   - main.go:              hello, world, NewLogger
//   - pkg/tokenizer.go:     BM25Tokenizer, BM25, tokens, scoring  (search e2e anchor)
//   - pkg/util.go:          upperCaseWords, ToUpper
//   - internal/logger.go:   Logger, logging, token
//   - internal/handler.go:  TokenHandler, BM25, token, pipeline
//
// Uses /tmp instead of t.TempDir to produce short paths: macOS limits AF_UNIX
// socket paths to 104 bytes, and t.TempDir paths easily exceed that limit.
func newTestProject(t *testing.T) string {
	t.Helper()
	root, err := os.MkdirTemp("/tmp", "idx-e2e")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	// git init creates a real repo so GitIgnoreMatcherFactory.verifyGitBinary passes.
	out, gitErr := exec.Command("git", "init", root).CombinedOutput()
	require.NoError(t, gitErr, "git init failed: %s", out)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "pkg"), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal"), 0o750))

	write := func(rel, content string) {
		require.NoError(t, os.WriteFile(filepath.Join(root, rel), []byte(content), 0o644))
	}
	write("main.go", "package main\n\nfunc main() {\n\thello := \"world\"\n\t_ = hello\n\tl := NewLogger()\n\t_ = l\n}\n")
	write("pkg/tokenizer.go", "package pkg\n\n// BM25Tokenizer splits text into tokens for BM25 scoring.\ntype BM25Tokenizer struct{}\n")
	write("pkg/util.go", "package pkg\n\nimport \"strings\"\n\nfunc upperCaseWords(s string) string {\n\treturn strings.ToUpper(s)\n}\n")
	write("internal/logger.go", "package internal\n\n// Logger handles structured logging for token events.\ntype Logger struct{}\n\nfunc NewLogger() *Logger { return &Logger{} }\n")
	write("internal/handler.go", "package internal\n\n// TokenHandler routes token requests to the BM25 pipeline.\ntype TokenHandler struct{ logger *Logger }\n\nfunc NewTokenHandler(l *Logger) *TokenHandler { return &TokenHandler{logger: l} }\n")
	return root
}

// startTestServer mirrors the DI from runServer() but uses a cancellable context
// so tests can shut down the server without OS signals. Creates .idx/ if absent.
// Loads .idx.yml from projectRoot so idx.config requests return correct data.
// The server goroutine is guaranteed to exit before t.Cleanup returns.
func startTestServer(t *testing.T, projectRoot string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, ".idx"), 0o750))

	configRepo := sharedconfig.NewYAMLRepository()
	cfg, overrides, _ := configRepo.Load(projectRoot)
	configFilePath := configRepo.FilePath(projectRoot)

	indexRepo := idxstorage.NewBinaryIndexRepository()
	checksumRepo := idxstorage.NewDirectoryChecksumRepository()
	readLogRepo := featread.NewReadLogRepository()
	serverDaemon := featdaemon.NewServerDaemonService(
		featdaemon.NewServerStateRepository(), nil, sharedoutput.NewLineWriter(io.Discard),
	)
	tuning := featsearch.SearchServiceOptions{BM25K1: 1.2, BM25B: 0.75, MaxWorkers: 1}

	socketPath := idxipc.SocketPath(projectRoot)
	srv := appserver.NewServer(appserver.ServerDeps{
		ProjectTree:     sharedfs.NewOSProjectTree(),
		MatcherFactory:  sharedfs.IgnoreMatcherBuilder(sharedfs.NewGitIgnoreMatcherFactory()),
		FileReader:      sharedfs.NewOSFileReader(),
		Indexer:         featindexing.NewBM25IndexService(),
		IndexRepo:       indexRepo,
		ChecksumRepo:    checksumRepo,
		DaemonRepo:      serverDaemon,
		ReadLogRepo:     readLogRepo,
		SearchTuning:    tuning,
		SocketPath:      socketPath,
		Config:          cfg,
		ConfigFilePath:  configFilePath,
		ConfigOverrides: overrides,
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_ = srv.Serve(ctx)
		close(done)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	waitForTestSocket(t, socketPath)
}

// waitForTestSocket polls until the Unix socket file appears.
// Mirrors waitForSocket in internal/app/server/handlers_test.go.
func waitForTestSocket(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("server socket %q did not appear within deadline", path)
}

// --- Group A: server-free commands ---

func TestCLI_Version_CompletesWithoutError(t *testing.T) {
	// Act
	err := run([]string{"idx", "version"}, io.Discard)

	// Assert
	require.NoError(t, err)
}

func TestCLI_ServerStatus_WhenNotRunning_OutputsMessage(t *testing.T) {
	// Arrange — point to a fresh project with no running daemon
	projectRoot := newTestProject(t)
	t.Setenv("IDX_PROJECT_PATH", projectRoot)
	var buf bytes.Buffer

	// Act
	err := run([]string{"idx", "server", "status"}, &buf)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, buf.String())
}

// --- Group B: full client→server pipeline ---
// These tests use t.Chdir so that both the client (via sharedDeps) and the
// server-side handlers (via ProjectTree.CurrentDir) resolve the same project root.
// t.Chdir cannot be used with t.Parallel.

func TestCLI_Init_CreatesIndexDirectory(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)

	// Act
	err := run([]string{"idx", "init"}, io.Discard)

	// Assert
	require.NoError(t, err)
	_, statErr := os.Stat(filepath.Join(projectRoot, ".idx"))
	assert.NoError(t, statErr, "expected .idx directory to exist after init")
}

func TestCLI_Status_AfterInit_OutputsStatus(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))
	var buf bytes.Buffer

	// Act
	err := run([]string{"idx", "status"}, &buf)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, buf.String())
}

func TestCLI_Search_AfterInit_ReturnsResult(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))
	var buf bytes.Buffer

	// Act — search for a term unique to pkg/tokenizer.go
	err := run([]string{"idx", "search", "BM25Tokenizer"}, &buf)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "tokenizer.go")
}

func TestCLI_Read_StreamsFileContents(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))
	var buf bytes.Buffer

	// Act
	err := run([]string{"idx", "read", filepath.Join(projectRoot, "main.go")}, &buf)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "hello")
}

func TestCLI_ConfigShow_CompletesWithoutError(t *testing.T) {
	// Arrange — idx config show now routes through the server.
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)

	// Act
	err := run([]string{"idx", "config", "show"}, io.Discard)

	// Assert
	require.NoError(t, err)
}
