package main

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureStdout redirects os.Stdout to a pipe while calling f, then returns
// everything written to stdout. Must not be used with t.Parallel — it mutates
// the global os.Stdout. Used here because idx version writes via cmd.OutOrStdout().
func captureStdout(t *testing.T, f func() error) (string, error) {
	t.Helper()
	saved := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	runErr := f()

	w.Close()
	os.Stdout = saved

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	return buf.String(), runErr
}

// --- idx version ---

func TestCLI_Version_OutputContainsBranding(t *testing.T) {
	// Arrange — version command writes to cmd.OutOrStdout() (os.Stdout, not d.writer).
	// captureStdout redirects os.Stdout so we can inspect the output.

	// Act
	out, err := captureStdout(t, func() error {
		return run([]string{"idx", "version"}, io.Discard)
	})

	// Assert
	require.NoError(t, err)
	assert.Contains(t, out, "IDX", "expected IDX branding in version output")
}

func TestCLI_Version_OutputContainsVersionString(t *testing.T) {
	// Arrange — build info version defaults to "dev" in tests (set via -ldflags in production)

	// Act
	out, err := captureStdout(t, func() error {
		return run([]string{"idx", "version"}, io.Discard)
	})

	// Assert
	require.NoError(t, err)
	// The version value ("dev" in tests) must appear somewhere in the output
	assert.Contains(t, out, "dev", "expected version string in version output")
}

func TestCLI_VersionFlag_OutputsVersionInfo(t *testing.T) {
	// `idx --version` uses cobra's version template, which also calls renderVersionOutput.

	// Act
	out, err := captureStdout(t, func() error {
		return run([]string{"idx", "--version"}, io.Discard)
	})

	// Assert — same branding as `idx version`
	require.NoError(t, err)
	assert.Contains(t, out, "IDX", "expected IDX branding from --version flag")
}

func TestCLI_Version_OutputContainsCurrentDirectory(t *testing.T) {
	// Arrange — the version panel shows a tilde-abbreviated current directory
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	// Act
	out, err := captureStdout(t, func() error {
		return run([]string{"idx", "version"}, io.Discard)
	})

	require.NoError(t, err)

	// Assert — either the full path or a tilde-abbreviated path is shown
	cwd, cwdErr := os.Getwd()
	require.NoError(t, cwdErr)
	hasCWD := false
	if len(cwd) > len(home) {
		hasCWD = assert.Contains(t, out, "~", "expected tilde-abbreviated path in version output")
	} else {
		hasCWD = assert.Contains(t, out, cwd)
	}
	_ = hasCWD
}

func TestCLI_Version_DoesNotRequireServer(t *testing.T) {
	// Arrange — deliberately no server running; IDX_PROJECT_PATH in an empty dir
	root, err := os.MkdirTemp("/tmp", "idx-e2e-ver-nosrv")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	t.Setenv("IDX_PROJECT_PATH", root)

	// Act
	err = run([]string{"idx", "version"}, io.Discard)

	// Assert — version is a local command; no server needed
	require.NoError(t, err)
}
