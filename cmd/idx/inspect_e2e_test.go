package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	idxtui "idx/internal/app/tui"
	featindexing "idx/internal/features/indexing"
)

// --- Argument validation (no server needed) ---

func TestCLI_Inspect_TooManyArgs_ReturnsValidationError(t *testing.T) {
	t.Parallel()

	// Act — inspect accepts at most one path argument
	err := run([]string{"idx", "inspect", "path/one", "path/two"}, io.Discard)

	// Assert
	require.Error(t, err)
	assert.ErrorContains(t, err, "inspect accepts at most one path")
}

// --- Error paths (server required) ---

func TestCLI_Inspect_BeforeInit_NoPath_ReturnsNoIndexError(t *testing.T) {
	// Arrange — server running, nothing indexed yet
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)

	// Act — no path triggers the "merge all directories" code path
	err := run([]string{"idx", "inspect"}, io.Discard)

	// Assert — expect a "no index found" error, not a crash or silent success
	require.Error(t, err)
	assert.ErrorContains(t, err, "idx init")
}

func TestCLI_Inspect_BeforeInit_WithPath_ReturnsNoIndexAtPathError(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)

	// Act — ask for the root directory's index which doesn't exist yet
	err := run([]string{"idx", "inspect", projectRoot}, io.Discard)

	// Assert — specific "no index found at <path>" error
	require.Error(t, err)
	assert.ErrorContains(t, err, "idx init")
}

func TestCLI_Inspect_WithNonExistentPath_ReturnsError(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Act — path does not correspond to an indexed directory
	ghostPath := filepath.Join(projectRoot, "does", "not", "exist")
	err := run([]string{"idx", "inspect", ghostPath}, io.Discard)

	// Assert
	require.Error(t, err)
}

func TestCLI_Inspect_WithoutServer_ReturnsServerNotRunningError(t *testing.T) {
	// Arrange — no server
	root, err := os.MkdirTemp("/tmp", "idx-e2e-inspect-nosrv")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	t.Setenv("IDX_PROJECT_PATH", root)

	// Act
	err = run([]string{"idx", "inspect"}, io.Discard)

	// Assert
	require.Error(t, err)
	assert.ErrorContains(t, err, "server not running")
}

// --- Happy path (TUI) ---

func TestCLI_Inspect_AfterInit_WithPath_PassesIndexToTUI(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Replace TUI runner with a no-op: the real BubbleTea program blocks reading
	// from terminal stdin and never exits when there is no TTY input to consume.
	var receivedIndex *featindexing.InvertedIndex
	idxtui.SetRunInspectTUITestHook(func(index *featindexing.InvertedIndex) error {
		receivedIndex = index
		return nil
	})
	t.Cleanup(func() { idxtui.SetRunInspectTUITestHook(nil) })

	// Act
	err := run([]string{"idx", "inspect", projectRoot}, io.Discard)

	// Assert — index was loaded and passed to the TUI runner without errors
	require.NoError(t, err)
	assert.NotNil(t, receivedIndex, "expected TUI runner to receive a non-nil index")
}
