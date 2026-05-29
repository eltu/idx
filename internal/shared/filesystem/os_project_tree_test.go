package filesystem

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestOSProjectTree_CurrentDir_ReturnsWorkingDirectory uses os.Chdir — not parallel-safe.
func TestOSProjectTree_CurrentDirAndFindGitRoot_LocatesRoot(t *testing.T) {
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(originalDir) })

	// Arrange
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, ".git"), []byte(""), 0600))
	nested := filepath.Join(root, "a", "b")
	require.NoError(t, os.MkdirAll(nested, 0750))
	require.NoError(t, os.Chdir(nested))

	tree := NewOSProjectTree()

	// Act
	cwd, err := tree.CurrentDir()
	require.NoError(t, err)

	// Assert — resolve symlinks so macOS /var → /private/var differences are handled
	resolvedExpected, err := filepath.EvalSymlinks(nested)
	require.NoError(t, err)
	resolvedCurrent, err := filepath.EvalSymlinks(cwd)
	require.NoError(t, err)
	assert.Equal(t, resolvedExpected, resolvedCurrent)

	gitRoot, err := tree.FindGitRoot(nested)
	require.NoError(t, err)
	assert.Equal(t, root, gitRoot)
}

func TestOSProjectTree_FindGitRoot_ReturnsErrorOutsideGitProject(t *testing.T) {
	t.Parallel()

	// Arrange
	tree := NewOSProjectTree()

	// Act
	_, err := tree.FindGitRoot(t.TempDir())

	// Assert
	require.Error(t, err)
}

func TestOSProjectTree_WriteExistsReadDirRemoveAll_LifecycleOperations(t *testing.T) {
	t.Parallel()

	// Arrange
	tree := NewOSProjectTree()
	root := t.TempDir()
	filePath := filepath.Join(root, "deep", "file.txt")

	// Act — write
	require.NoError(t, tree.WriteFile(filePath, []byte("content")))

	// Assert — exists
	exists, err := tree.Exists(filePath)
	require.NoError(t, err)
	assert.True(t, exists)

	// Assert — readdir
	entries, err := tree.ReadDir(filepath.Join(root, "deep"))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "file.txt", entries[0].Name)
	assert.False(t, entries[0].IsDir)
	assert.NotZero(t, entries[0].Size)
	assert.NotZero(t, entries[0].ModTimeUnixNano)

	// Act — remove all
	require.NoError(t, tree.RemoveAll(filepath.Join(root, "deep")))

	// Assert — file gone
	exists, err = tree.Exists(filePath)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestOSProjectTree_ErrorBranches_ReturnsErrorForInvalidPaths(t *testing.T) {
	t.Parallel()

	// Arrange
	tree := NewOSProjectTree()

	// Assert — ReadDir missing path
	_, err := tree.ReadDir(filepath.Join(t.TempDir(), "missing"))
	require.Error(t, err)

	// Assert — Exists invalid path
	_, err = tree.Exists("\x00invalid")
	require.Error(t, err)

	// Assert — RemoveAll invalid path
	require.Error(t, tree.RemoveAll("\x00invalid"))

	// Assert — WriteFile invalid path
	require.Error(t, tree.WriteFile("\x00invalid", []byte("x")))
}

func TestOSProjectTree_ReadDir_MarksSymlinkEntries(t *testing.T) {
	t.Parallel()

	// Arrange
	tree := NewOSProjectTree()
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "target.txt"), []byte("content"), 0600))
	require.NoError(t, os.Symlink("target.txt", filepath.Join(root, "link.txt")))

	// Act
	entries, err := tree.ReadDir(root)
	require.NoError(t, err)

	// Assert
	var found bool
	for _, entry := range entries {
		if entry.Name == "link.txt" {
			found = true
			assert.True(t, entry.IsSymlink, "expected symlink entry IsSymlink=true")
		}
	}
	assert.True(t, found, "expected link.txt in directory entries")
}
