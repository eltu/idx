package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runStatus executes `idx status [flags]` and returns captured output and error.
func runStatus(t *testing.T, flags ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	err := run(append([]string{"idx", "status"}, flags...), &buf)
	return buf.String(), err
}

// --- Happy paths ---

func TestCLI_Status_AfterInit_OutputsSuccessPanel(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Act
	out, err := runStatus(t)

	// Assert — status panel should be non-empty and carry no errors
	require.NoError(t, err)
	assert.NotEmpty(t, out)
}

func TestCLI_Status_AfterInit_OutputContainsProjectInfo(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Act
	out, err := runStatus(t)

	// Assert — status panel includes index metadata
	require.NoError(t, err)
	// The project directory base name appears in the status output
	assert.Contains(t, out, filepath.Base(projectRoot))
}

func TestCLI_Status_AfterSyncWithNoChanges_ShowsFreshIndex(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))
	require.NoError(t, run([]string{"idx", "sync"}, io.Discard))

	// Act
	out, err := runStatus(t)

	// Assert — fresh index should not report stale directories
	require.NoError(t, err)
	assert.NotContains(t, out, "stale", "expected no stale directories after sync")
}

// --- Stale detection ---

func TestCLI_Status_AfterModifyingFile_ReportsStaleIndex(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Modify a file to make the checksum differ from the stored snapshot
	modified := "package main\n\n// modified content\nfunc main() {}\n"
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "main.go"), []byte(modified), 0o644))

	// Act
	out, err := runStatus(t)

	// Assert — status must detect the checksum mismatch
	require.NoError(t, err)
	assert.True(t,
		containsAny(out, "stale", "sync", "❌"),
		"expected status to report a stale index after file modification, got: %q", out,
	)
}

// --- Error paths ---

func TestCLI_Status_BeforeInit_ReturnsNoIndexError(t *testing.T) {
	// Arrange — server running, project NOT initialized
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)

	// Act
	out, err := runStatus(t)

	// Assert — CommandResponse wraps the service error; client prints output
	require.NoError(t, err, "handleStatus returns CommandResponse, not RPC error")
	assert.Contains(t, out, "idx init", "expected output to mention idx init")
}

func TestCLI_Status_WithoutServer_ReturnsServerNotRunningError(t *testing.T) {
	// Arrange — no server
	root, err := os.MkdirTemp("/tmp", "idx-e2e-status-nosrv")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	t.Setenv("IDX_PROJECT_PATH", root)

	// Act
	_, err = runStatus(t)

	// Assert
	require.Error(t, err)
	assert.ErrorContains(t, err, "not running")
}
