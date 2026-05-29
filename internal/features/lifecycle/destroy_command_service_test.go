package lifecycle_test

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	lifecycle "idx/internal/features/lifecycle"
	"idx/internal/shared/filesystem"
)

func TestDestroyCommandService_Run_RemovesIdxDirectoriesRecursively(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	apiDir := filepath.Join(rootDir, "cmd", "api")
	coreDir := filepath.Join(rootDir, "internal", "core")

	tree := newFakeProjectTree(rootDir, rootDir)
	tree.readDirMap[rootDir] = []filesystem.DirectoryEntry{
		{Name: ".git", Path: filepath.Join(rootDir, ".git"), IsDir: true},
		{Name: ".idx", Path: filepath.Join(rootDir, ".idx"), IsDir: true},
		{Name: "cmd", Path: filepath.Join(rootDir, "cmd"), IsDir: true},
		{Name: "internal", Path: filepath.Join(rootDir, "internal"), IsDir: true},
	}
	tree.readDirMap[filepath.Join(rootDir, "cmd")] = []filesystem.DirectoryEntry{{Name: "api", Path: apiDir, IsDir: true}}
	tree.readDirMap[apiDir] = []filesystem.DirectoryEntry{{Name: ".idx", Path: filepath.Join(apiDir, ".idx"), IsDir: true}}
	tree.readDirMap[filepath.Join(rootDir, "internal")] = []filesystem.DirectoryEntry{{Name: "core", Path: coreDir, IsDir: true}}
	tree.readDirMap[coreDir] = []filesystem.DirectoryEntry{{Name: ".idx", Path: filepath.Join(coreDir, ".idx"), IsDir: true}}
	output := &capturingTextOutput{}
	service := lifecycle.NewDestroyCommandService(tree, output)

	// Act
	err := service.Run()

	// Assert
	require.NoError(t, err)
	assert.Len(t, tree.removed, 3)
	assert.Equal(t, filepath.Join(rootDir, ".idx"), tree.removed[0])
	require.Len(t, output.lines, 1)
	assert.Equal(t, "🧹 Index metadata removed from project.", output.lines[0])
}

func TestDestroyCommandService_Run_RequiresProjectRoot(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	currentDir := filepath.Join(rootDir, "internal")
	tree := newFakeProjectTree(currentDir, rootDir)
	service := lifecycle.NewDestroyCommandService(tree, &capturingTextOutput{})

	// Act
	err := service.Run()

	// Assert
	require.Error(t, err)
	assert.EqualError(t, err, `destroy must run from project root: got current directory "/repo/internal", expected root directory "/repo"`)
	assert.Empty(t, tree.removed)
}

func TestDestroyCommandService_Run_FailsWhenCurrentDirResolutionFails(t *testing.T) {
	t.Parallel()

	// Arrange
	tree := newFakeProjectTree("", "/repo")
	tree.gitRootErr = errors.New("git root unavailable")
	service := lifecycle.NewDestroyCommandService(tree, &capturingTextOutput{})

	// Act
	err := service.Run()

	// Assert
	require.Error(t, err)
}

func TestDestroyCommandService_Run_FailsWhenGitRootLookupFails(t *testing.T) {
	t.Parallel()

	// Arrange
	tree := newFakeProjectTree("/repo", "/repo")
	tree.gitRootErr = errors.New("not a git repository")
	service := lifecycle.NewDestroyCommandService(tree, &capturingTextOutput{})

	// Act
	err := service.Run()

	// Assert
	require.Error(t, err)
}

func TestDestroyCommandService_Run_ReturnsErrorWhenDependenciesAreNil(t *testing.T) {
	t.Parallel()

	// Arrange
	service := lifecycle.NewDestroyCommandService(nil, nil)

	// Act & Assert
	require.Error(t, service.Run())
}

func TestDestroyCommandService_Run_ContinuesAfterRemoveFailureAndReturnsError(t *testing.T) {
	t.Parallel()

	// Arrange
	rootDir := filepath.Join(string(filepath.Separator), "repo")
	apiDir := filepath.Join(rootDir, "cmd", "api")
	coreDir := filepath.Join(rootDir, "internal", "core")
	failingIdx := filepath.Join(apiDir, ".idx")

	tree := newFakeProjectTree(rootDir, rootDir)
	tree.readDirMap[rootDir] = []filesystem.DirectoryEntry{
		{Name: ".git", Path: filepath.Join(rootDir, ".git"), IsDir: true},
		{Name: ".idx", Path: filepath.Join(rootDir, ".idx"), IsDir: true},
		{Name: "cmd", Path: filepath.Join(rootDir, "cmd"), IsDir: true},
		{Name: "internal", Path: filepath.Join(rootDir, "internal"), IsDir: true},
	}
	tree.readDirMap[filepath.Join(rootDir, "cmd")] = []filesystem.DirectoryEntry{{Name: "api", Path: apiDir, IsDir: true}}
	tree.readDirMap[apiDir] = []filesystem.DirectoryEntry{{Name: ".idx", Path: failingIdx, IsDir: true}}
	tree.readDirMap[filepath.Join(rootDir, "internal")] = []filesystem.DirectoryEntry{{Name: "core", Path: coreDir, IsDir: true}}
	tree.readDirMap[coreDir] = []filesystem.DirectoryEntry{{Name: ".idx", Path: filepath.Join(coreDir, ".idx"), IsDir: true}}
	tree.removeErrs[failingIdx] = errors.New("permission denied")
	output := &capturingTextOutput{}
	service := lifecycle.NewDestroyCommandService(tree, output)

	// Act
	err := service.Run()

	// Assert
	require.Error(t, err)
	assert.Len(t, tree.removed, 3)
	assert.ErrorContains(t, err, failingIdx)
	assert.Empty(t, output.lines)
}

// fakeProjectTree implements filesystem.ProjectTree for testing.
type fakeProjectTree struct {
	currentDir string
	gitRoot    string
	readDirMap map[string][]filesystem.DirectoryEntry
	readDirErr map[string]error
	existing   map[string]bool
	removed    []string
	removeErrs map[string]error
	writes     map[string]string
	gitRootErr error
}

func newFakeProjectTree(currentDir, gitRoot string) *fakeProjectTree {
	return &fakeProjectTree{
		currentDir: currentDir,
		gitRoot:    gitRoot,
		readDirMap: map[string][]filesystem.DirectoryEntry{},
		readDirErr: map[string]error{},
		existing:   map[string]bool{},
		removed:    []string{},
		removeErrs: map[string]error{},
		writes:     map[string]string{},
	}
}

func (tree *fakeProjectTree) CurrentDir() (string, error) {
	if tree.currentDir == "" {
		return "", errors.New("cwd unavailable")
	}
	return tree.currentDir, nil
}

func (tree *fakeProjectTree) FindGitRoot(_ string) (string, error) {
	if tree.gitRootErr != nil {
		return "", tree.gitRootErr
	}
	return tree.gitRoot, nil
}

func (tree *fakeProjectTree) ReadDir(path string) ([]filesystem.DirectoryEntry, error) {
	if err, ok := tree.readDirErr[path]; ok {
		return nil, err
	}
	entries, ok := tree.readDirMap[path]
	if !ok {
		return []filesystem.DirectoryEntry{}, nil
	}
	return entries, nil
}

func (tree *fakeProjectTree) Exists(path string) (bool, error) {
	return tree.existing[path], nil
}

func (tree *fakeProjectTree) RemoveAll(path string) error {
	tree.removed = append(tree.removed, path)
	if err, hasError := tree.removeErrs[path]; hasError {
		return err
	}
	return nil
}

func (tree *fakeProjectTree) WriteFile(path string, content []byte) error {
	tree.writes[path] = string(content)
	return nil
}

type capturingTextOutput struct {
	lines []string
}

func (output *capturingTextOutput) WriteLine(text string) error {
	output.lines = append(output.lines, text)
	return nil
}
