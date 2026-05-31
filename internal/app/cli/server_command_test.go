package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- findProjectRoot ---

func TestFindProjectRoot_FromRootItself_ReturnsRoot(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".idx"), 0o750))

	// Act
	got, err := findProjectRoot(dir)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, dir, got)
}

func TestFindProjectRoot_FromSubdirectory_FindsRoot(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	nested := filepath.Join(root, "internal", "core", "pkg")
	require.NoError(t, os.MkdirAll(nested, 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".idx"), 0o750))

	// Act
	got, err := findProjectRoot(nested)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, root, got)
}

func TestFindProjectRoot_NoDotIdx_ReturnsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir() // no .idx directory
	_, err := findProjectRoot(dir)
	require.Error(t, err)
}

func TestFindProjectRoot_NestedDotIdx_StopsAtClosestAncestor(t *testing.T) {
	t.Parallel()

	// Two nested .idx directories — should stop at the closest ancestor.
	outer := t.TempDir()
	inner := filepath.Join(outer, "sub")
	require.NoError(t, os.MkdirAll(filepath.Join(inner, ".idx"), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(outer, ".idx"), 0o750))

	got, err := findProjectRoot(inner)
	require.NoError(t, err)
	assert.Equal(t, inner, got)
}

// ---- server status --json flag ----

func TestNewServerStatusCommand_HasJsonFlag(t *testing.T) {
	t.Parallel()

	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).
		WithServerManager(&stubServerManager{})
	cmd := runner.newServerStatusCommand()
	assert.NotNil(t, cmd.Flags().Lookup("json"), "expected --json flag on server status")
}

func TestNewServerStatusCommand_JsonShorthand_RegisteredAsJ(t *testing.T) {
	t.Parallel()

	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).
		WithServerManager(&stubServerManager{})
	cmd := runner.newServerStatusCommand()
	assert.NotNil(t, cmd.Flags().ShorthandLookup("j"), "expected -j shorthand for --json")
}
