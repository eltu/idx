package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runRead executes `idx read <path> [flags]` and returns captured output and error.
func runRead(t *testing.T, path string, flags ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	args := append([]string{"idx", "read", path}, flags...)
	err := run(args, &buf)
	return buf.String(), err
}

// --- Argument validation (no server needed) ---

func TestCLI_Read_MissingPath_ReturnsArgsError(t *testing.T) {
	t.Parallel()

	// Act — cobra ExactArgs(1) rejects zero arguments
	err := run([]string{"idx", "read"}, io.Discard)

	// Assert
	require.Error(t, err)
}

// --- Happy paths ---

func TestCLI_Read_StreamsAllLines(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))
	target := filepath.Join(projectRoot, "main.go")

	// Act
	out, err := runRead(t, target)

	// Assert — full file contents are returned
	require.NoError(t, err)
	assert.Contains(t, out, "package main")
	assert.Contains(t, out, "hello")
}

func TestCLI_Read_WithToFlag_LimitsOutput(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))
	// main.go line 1 is "package main"
	target := filepath.Join(projectRoot, "main.go")

	// Act — read only the first line
	out, err := runRead(t, target, "--to", "1")

	// Assert — only line 1 is present; "hello" is further down
	require.NoError(t, err)
	assert.Contains(t, out, "package main")
	assert.NotContains(t, out, "hello", "expected --to 1 to exclude lines after the first")
}

func TestCLI_Read_WithFromFlag_StartsAtGivenLine(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))
	target := filepath.Join(projectRoot, "main.go")

	// Act — skip line 1 ("package main") entirely
	out, err := runRead(t, target, "--from", "2")

	// Assert — line 1 is absent; rest of the file is present
	require.NoError(t, err)
	assert.NotContains(t, out, "package main", "expected --from 2 to skip the first line")
	assert.Contains(t, out, "hello", "expected lines after line 1 to be present")
}

func TestCLI_Read_WithFromAndToFlags_ReturnsExactRange(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))
	// pkg/tokenizer.go: line 1="package pkg", line 3="// BM25Tokenizer..."
	target := filepath.Join(projectRoot, "pkg", "tokenizer.go")

	// Act — read a single line in the middle
	out, err := runRead(t, target, "--from", "3", "--to", "3")

	// Assert — only line 3 appears (the comment line with BM25Tokenizer)
	require.NoError(t, err)
	assert.Contains(t, out, "BM25Tokenizer")
	assert.NotContains(t, out, "package pkg", "expected --from 3 to exclude line 1")
}

func TestCLI_Read_NonExistentFile_ReturnsEmptyOutput(t *testing.T) {
	// Arrange — handleRead swallows file-not-found errors and returns empty lines
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)

	// Act
	out, err := runRead(t, filepath.Join(projectRoot, "ghost.go"))

	// Assert — no error propagated to client; output is empty
	require.NoError(t, err)
	assert.Empty(t, strings.TrimSpace(out))
}

func TestCLI_Read_LogsAccessForPopularityBoost(t *testing.T) {
	// Arrange — after reading a file, the read log must contain an entry for it.
	// The next search gives that file a popularity boost (verified indirectly by checking
	// the read log file was created, not by ranking comparison which is non-deterministic).
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))
	target := filepath.Join(projectRoot, "main.go")

	// Act
	_, err := runRead(t, target)
	require.NoError(t, err)

	// Assert — .idx/read_log.idx must exist after a successful read
	readLogPath := filepath.Join(projectRoot, ".idx", "read_log.idx")
	_, statErr := os.Stat(readLogPath)
	assert.NoError(t, statErr, "expected read_log.idx to be created after idx read")
}

// --- Error paths ---

func TestCLI_Read_WithoutServer_ReturnsServerNotRunningError(t *testing.T) {
	// Arrange — no server
	root, err := os.MkdirTemp("/tmp", "idx-e2e-read-nosrv")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	t.Setenv("IDX_PROJECT_PATH", root)

	// Act
	err = run([]string{"idx", "read", "/any/path.go"}, io.Discard)

	// Assert
	require.Error(t, err)
	assert.ErrorContains(t, err, "server not running")
}

// --- New UX: open alias, --start/--end flags ---

func TestCLI_Read_OpenAlias_SameAsRead(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))
	target := filepath.Join(projectRoot, "main.go")

	// Act
	var buf1, buf2 bytes.Buffer
	require.NoError(t, run([]string{"idx", "open", target}, &buf1))
	require.NoError(t, run([]string{"idx", "read", target}, &buf2))

	// Assert — alias produces identical output
	assert.Equal(t, buf2.String(), buf1.String())
}

func TestCLI_Read_StartEndFlags_SameLineRange(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))
	target := filepath.Join(projectRoot, "main.go")

	// Act — new flags vs deprecated flags
	out1, err1 := runRead(t, target, "--start", "1", "--end", "3")
	out2, err2 := runRead(t, target, "--from", "1", "--to", "3")

	// Assert — same content for the same line range
	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.Equal(t, out2, out1)
}

func TestCLI_Read_StartShorthand_Works(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))
	target := filepath.Join(projectRoot, "main.go")

	// Act — -s for --start, -e for --end
	out, err := runRead(t, target, "-s", "1", "-e", "2")

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, out)
}
