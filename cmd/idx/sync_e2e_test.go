package main

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Happy paths ---

func TestCLI_Sync_AfterInit_NoChanges_Succeeds(t *testing.T) {
	// Arrange — nothing changed since init
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Act
	err := run([]string{"idx", "sync"}, io.Discard)

	// Assert — sync is idempotent; no changes means no-op, no error
	require.NoError(t, err)
}

func TestCLI_Sync_AfterModifyingFile_ReindexesChangedContent(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Add a unique term to main.go after init
	newContent := "package main\n\n// SyncUniqueMarker is a post-init addition.\nfunc SyncUniqueMarker() {}\n"
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "main.go"), []byte(newContent), 0o644))

	// Act
	require.NoError(t, run([]string{"idx", "sync"}, io.Discard))

	// Assert — unique term is now searchable after sync
	out, err := runSearch(t, "SyncUniqueMarker", "--format", "json")
	require.NoError(t, err)
	resp := parseSearchJSON(t, out)
	assert.GreaterOrEqual(t, resp.Count, 1, "expected SyncUniqueMarker to be findable after sync")
}

func TestCLI_Sync_AfterAddingNewFile_IndexesNewFile(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Write a new file with a unique term
	newFile := filepath.Join(projectRoot, "pkg", "newfile.go")
	require.NoError(t, os.WriteFile(newFile, []byte("package pkg\n\n// NewFileSyncTermXYZ is the unique term.\nfunc NewFileSyncTermXYZ() {}\n"), 0o644))

	// Act
	require.NoError(t, run([]string{"idx", "sync"}, io.Discard))

	// Assert
	out, err := runSearch(t, "NewFileSyncTermXYZ", "--format", "json")
	require.NoError(t, err)
	resp := parseSearchJSON(t, out)
	assert.GreaterOrEqual(t, resp.Count, 1)
}

// --- Error paths ---

func TestCLI_Sync_BeforeInit_ReturnsNotIndexedError(t *testing.T) {
	// Arrange — git project, no idx init
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)

	// Act
	var out string
	err := run([]string{"idx", "sync"}, writerFunc(func(s string) { out += s }))

	// Assert — sync returns success=false with "run idx init first" message
	// (handleSync wraps the error in CommandResponse, client prints the output)
	require.NoError(t, err, "handleSync returns CommandResponse, not RPC error")
	assert.Contains(t, out, "idx init", "expected output to mention idx init")
}

func TestCLI_Sync_FromSubdirectory_ReturnsProjectRootError(t *testing.T) {
	// Arrange — init from root, then chdir into subdirectory
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))
	t.Chdir(filepath.Join(projectRoot, "pkg"))

	// Act
	var out string
	err := run([]string{"idx", "sync"}, writerFunc(func(s string) { out += s }))

	// Assert — sync detects it's not at project root
	require.NoError(t, err)
	assert.Contains(t, out, "project root", "expected output to mention project root")
}

func TestCLI_Sync_WithoutServer_ReturnsServerNotRunningError(t *testing.T) {
	// Arrange — no server
	root, err := os.MkdirTemp("/tmp", "idx-e2e-nosrv")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	t.Setenv("IDX_PROJECT_PATH", root)

	// Act
	err = run([]string{"idx", "sync"}, io.Discard)

	// Assert
	require.Error(t, err)
	assert.ErrorContains(t, err, "server not running")
}

// writerFunc adapts a func(string) to the io.Writer interface for output capture.
type writerFunc func(string)

func (f writerFunc) Write(p []byte) (int, error) {
	f(string(p))
	return len(p), nil
}

// --- New UX: update alias ---

func TestCLI_Sync_UpdateAlias_RunsSync(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Act — idx update is an alias for idx sync
	err := run([]string{"idx", "update"}, io.Discard)

	// Assert — alias succeeds without error
	require.NoError(t, err)
}
