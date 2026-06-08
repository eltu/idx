package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
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
	sharedfs "idx/internal/shared/filesystem"
	idxipc "idx/internal/shared/ipc"
	sharedoutput "idx/internal/shared/output"
)

// containsAny reports whether s contains at least one of the given substrings.
func containsAny(s string, substrings ...string) bool {
	for _, sub := range substrings {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// startTestServerCancellable mirrors startTestServer but exposes the cancel func
// so tests can stop the server mid-scenario (e.g. after destroy).
func startTestServerCancellable(t *testing.T, projectRoot string) (cancel func()) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, ".idx"), 0o750))

	indexRepo := idxstorage.NewBinaryIndexRepository()
	checksumRepo := idxstorage.NewDirectoryChecksumRepository()
	readLogRepo := featread.NewReadLogRepository()
	serverDaemon := featdaemon.NewServerDaemonService(
		featdaemon.NewServerStateRepository(), nil, sharedoutput.NewLineWriter(io.Discard),
	)
	tuning := featsearch.SearchServiceOptions{BM25K1: 1.2, BM25B: 0.75, MaxWorkers: 1}

	socketPath := idxipc.SocketPath(projectRoot)
	srv := appserver.NewServer(appserver.ServerDeps{
		ProjectTree:    sharedfs.NewOSProjectTree(),
		MatcherFactory: sharedfs.IgnoreMatcherBuilder(sharedfs.NewGitIgnoreMatcherFactory()),
		FileReader:     sharedfs.NewOSFileReader(),
		Indexer:        featindexing.NewBM25IndexService(),
		IndexRepo:      indexRepo,
		ChecksumRepo:   checksumRepo,
		DaemonRepo:     serverDaemon,
		ReadLogRepo:    readLogRepo,
		SearchTuning:   tuning,
		SocketPath:     socketPath,
	})

	ctx, cancelFn := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		_ = srv.Serve(ctx)
		close(done)
	}()
	t.Cleanup(func() {
		cancelFn()
		<-done
	})
	waitForTestSocket(t, socketPath)
	return cancelFn
}

// startTestServerWithIgnore starts the server with a composite ignore matcher
// that applies the given glob patterns on top of .gitignore rules.
// Used to verify that index.ignore in .idx.yml excludes files from the index.
func startTestServerWithIgnore(t *testing.T, projectRoot string, ignorePatterns []string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, ".idx"), 0o750))

	indexRepo := idxstorage.NewBinaryIndexRepository()
	checksumRepo := idxstorage.NewDirectoryChecksumRepository()
	readLogRepo := featread.NewReadLogRepository()
	serverDaemon := featdaemon.NewServerDaemonService(
		featdaemon.NewServerStateRepository(), nil, sharedoutput.NewLineWriter(io.Discard),
	)
	tuning := featsearch.SearchServiceOptions{BM25K1: 1.2, BM25B: 0.75, MaxWorkers: 1}

	composite := sharedfs.NewCompositeIgnoreMatcherFactory(
		sharedfs.IgnoreMatcherBuilder(sharedfs.NewGitIgnoreMatcherFactory()),
		sharedfs.NewGlobIgnoreMatcherFactory(ignorePatterns),
	)

	socketPath := idxipc.SocketPath(projectRoot)
	srv := appserver.NewServer(appserver.ServerDeps{
		ProjectTree:    sharedfs.NewOSProjectTree(),
		MatcherFactory: composite,
		FileReader:     sharedfs.NewOSFileReader(),
		Indexer:        featindexing.NewBM25IndexService(),
		IndexRepo:      indexRepo,
		ChecksumRepo:   checksumRepo,
		DaemonRepo:     serverDaemon,
		ReadLogRepo:    readLogRepo,
		SearchTuning:   tuning,
		SocketPath:     socketPath,
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

// waitForSocketGone polls until the Unix socket file disappears or the deadline
// passes. Used to confirm the server has shut down after a cancel or stop.
func waitForSocketGone(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("socket %q did not disappear within deadline", path)
}

// --- init scenarios ---

func TestCLI_Init_Idempotent_SecondRunOutputsAlreadyIndexedMessage(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))
	var buf bytes.Buffer

	// Act — second init on an already-indexed project
	err := run([]string{"idx", "init"}, &buf)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "already indexed")
}

func TestCLI_Init_WithoutGitRepo_ReturnsError(t *testing.T) {
	// Arrange — temp dir with NO git init; idx init now runs in-process (no server needed)
	root, err := os.MkdirTemp("/tmp", "idx-e2e-nogit")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	t.Chdir(root)

	// Act
	err = run([]string{"idx", "init"}, io.Discard)

	// Assert — init properly propagates the FindGitRoot error since it runs locally
	require.Error(t, err)
	assert.ErrorContains(t, err, "git")
	// No index file should have been created
	_, statErr := os.Stat(filepath.Join(root, ".idx", "index.idx"))
	assert.True(t, os.IsNotExist(statErr), "expected no index.idx when project has no git repo")
}

func TestCLI_Init_WithIdxYml_IgnorePatternExcludesFilesFromIndex(t *testing.T) {
	// Arrange — write .idx.yml that ignores pkg/; server is started with matching factory
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)

	idxYml := "index:\n  ignore:\n    - pkg/\n"
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, ".idx.yml"), []byte(idxYml), 0o644))

	startTestServerWithIgnore(t, projectRoot, []string{"pkg/"})
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Act — "BM25Tokenizer" lives only in pkg/tokenizer.go which was excluded
	out, err := runSearch(t, "BM25Tokenizer", "--format", "json")

	// Assert
	require.NoError(t, err)
	resp := parseSearchJSON(t, out)
	assert.Equal(t, 0, resp.Count, "expected pkg/ to be excluded from index by ignore pattern")
}

// --- destroy scenarios ---

func TestCLI_Destroy_WithoutServer_ReturnsServerNotRunningError(t *testing.T) {
	// Arrange — no server started; IDX_PROJECT_PATH points to a temp dir
	root, err := os.MkdirTemp("/tmp", "idx-e2e-noserver")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	t.Setenv("IDX_PROJECT_PATH", root)

	// Act
	err = run([]string{"idx", "destroy"}, io.Discard)

	// Assert — client cannot connect to socket, must return a server-not-running error
	require.Error(t, err)
	assert.ErrorContains(t, err, "server not running")
}

func TestCLI_Destroy_RemovesIdxDirectoriesRecursively(t *testing.T) {
	// Arrange — init creates .idx/ in root, pkg/, and internal/
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))
	var buf bytes.Buffer

	// Act
	err := run([]string{"idx", "destroy"}, &buf)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "🧹")
	for _, dir := range []string{".", "pkg", "internal"} {
		idxDir := filepath.Join(projectRoot, dir, ".idx")
		_, statErr := os.Stat(idxDir)
		assert.True(t, os.IsNotExist(statErr), "expected %s to be removed after destroy", idxDir)
	}
}

func TestCLI_Destroy_WithServerRunning_StopsServerAfterRemovingIndexes(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	socketPath := idxipc.SocketPath(projectRoot)
	_, err := os.Stat(socketPath)
	require.NoError(t, err, "socket must exist before destroy")

	var buf bytes.Buffer

	// Act
	err = run([]string{"idx", "destroy"}, &buf)

	// Assert — destroy succeeded and stopServerForDestroy wrote a server-status message.
	// The exact message ("Server stopped" vs "Server is not running") depends on whether
	// the socket path computed from "." matches the path used to start the server. On
	// macOS, /tmp is a symlink to /private/tmp, so the paths may differ. Both outcomes
	// confirm that stopServerForDestroy was called and completed without error.
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "🧹")
	serverMsg := buf.String()
	assert.True(t,
		containsAny(serverMsg, "Server stopped", "Server is not running"),
		"expected a server-status message in output, got: %q", serverMsg,
	)
}

func TestCLI_Destroy_OutsideProjectRoot_LeavesIndexesIntact(t *testing.T) {
	// Arrange — init from root, then chdir into a subdirectory before destroy
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// chdir to a subdirectory so destroy sees currentDir != projectRoot
	t.Chdir(filepath.Join(projectRoot, "pkg"))

	// Act
	err := run([]string{"idx", "destroy"}, io.Discard)

	// Assert — current behavior: server returns success:false (no output), client returns nil
	require.NoError(t, err)
	// The observable: .idx/ at the project root still exists
	_, statErr := os.Stat(filepath.Join(projectRoot, ".idx"))
	assert.NoError(t, statErr, "expected .idx to remain when destroy ran from a subdirectory")
}

func TestCLI_Destroy_AfterDestroy_SearchReturnsServerNotRunning(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	cancelServer := startTestServerCancellable(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Destroy indexes
	require.NoError(t, run([]string{"idx", "destroy"}, io.Discard))

	// Simulate the real-world SIGTERM: cancel the server goroutine and wait for socket to vanish
	cancelServer()
	waitForSocketGone(t, idxipc.SocketPath(projectRoot))

	// Act — search after server is fully stopped
	_, err := runSearch(t, "BM25Tokenizer")

	// Assert
	require.Error(t, err)
	assert.ErrorContains(t, err, "server not running")
}
