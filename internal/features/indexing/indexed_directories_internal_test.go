package indexing

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"idx/internal/shared/filesystem"
)

const testRepoRoot = "/repo"

type indexedTreeStub struct {
	entries map[string][]filesystem.DirectoryEntry
	exists  map[string]bool
	errDir  map[string]error
	errStat map[string]error
}

func (tree indexedTreeStub) CurrentDir() (string, error)        { return "", nil }
func (tree indexedTreeStub) FindGitRoot(string) (string, error) { return "", nil }
func (tree indexedTreeStub) RemoveAll(string) error             { return nil }
func (tree indexedTreeStub) WriteFile(string, []byte) error     { return nil }

func (tree indexedTreeStub) ReadDir(path string) ([]filesystem.DirectoryEntry, error) {
	if err := tree.errDir[path]; err != nil {
		return nil, err
	}
	return tree.entries[path], nil
}

func (tree indexedTreeStub) Exists(path string) (bool, error) {
	if err := tree.errStat[path]; err != nil {
		return false, err
	}
	return tree.exists[path], nil
}

type allowAllMatcher struct{}

func (allowAllMatcher) Matches(string) (bool, error) { return false, nil }

func TestIndexedDirectories_AndEligibleDirectories_ReturnsIndexedAndEligible(t *testing.T) {
	t.Parallel()

	// Arrange
	root := testRepoRoot
	child := filepath.Join(root, "child")
	tree := indexedTreeStub{
		entries: map[string][]filesystem.DirectoryEntry{
			root: {
				{Name: ".git", Path: filepath.Join(root, ".git"), IsDir: true},
				{Name: ".idx", Path: filepath.Join(root, ".idx"), IsDir: true},
				{Name: "child", Path: child, IsDir: true},
			},
			child: {{Name: "file.txt", Path: filepath.Join(child, "file.txt"), IsDir: false}},
		},
		exists: map[string]bool{
			filepath.Join(root, ".idx", "index.idx"):  true,
			filepath.Join(child, ".idx", "index.idx"): true,
		},
		errDir:  map[string]error{},
		errStat: map[string]error{},
	}

	// Act
	indexed, err := IndexedDirectories(tree, root)

	// Assert
	require.NoError(t, err)
	assert.Len(t, indexed, 2)

	eligible, err := eligibleDirectories(tree, root, allowAllMatcher{})
	require.NoError(t, err)
	assert.Len(t, eligible, 2)
}

func TestIndexedDirectories_StatError_ReturnsError(t *testing.T) {
	t.Parallel()

	// Arrange
	root := testRepoRoot
	treeWithStatError := indexedTreeStub{
		entries: map[string][]filesystem.DirectoryEntry{root: {}},
		exists:  map[string]bool{},
		errDir:  map[string]error{},
		errStat: map[string]error{filepath.Join(root, ".idx", "index.idx"): errors.New("stat failed")},
	}

	// Act
	_, err := IndexedDirectories(treeWithStatError, root)

	// Assert
	require.Error(t, err)
}

func TestIndexedDirectories_ReadDirError_ReturnsError(t *testing.T) {
	t.Parallel()

	// Arrange
	root := testRepoRoot
	treeWithReadError := indexedTreeStub{
		entries: map[string][]filesystem.DirectoryEntry{},
		exists:  map[string]bool{},
		errDir:  map[string]error{root: errors.New("read failed")},
		errStat: map[string]error{},
	}

	// Act
	_, err := IndexedDirectories(treeWithReadError, root)

	// Assert
	require.Error(t, err)
}
