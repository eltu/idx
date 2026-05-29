package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoverInspectTransactionLogFiles_FindsAllDirectories(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()
	paths := []string{
		filepath.Join(root, ".idx", "logs", "tlog.idx"),
		filepath.Join(root, "internal", ".idx", "logs", "tlog.idx"),
		filepath.Join(root, "cmd", "idx", "logs", "tlog.idx"),
	}
	for _, path := range paths {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte("{}\n"), 0o644))
	}

	// Act
	found, err := discoverInspectTransactionLogFiles(root)

	// Assert
	require.NoError(t, err)
	assert.Len(t, found, 3)
}
