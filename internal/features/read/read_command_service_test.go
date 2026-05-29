package read_test

import (
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"idx/internal/features/read"
	"idx/internal/shared/filesystem"
)

const (
	pkgMainContent = "package main\n"
	pkgFooContent  = "package foo\n"
)

// ---------------------------------------------------------------------------
// Happy path — basic reads
// ---------------------------------------------------------------------------

func TestReadCommandService_Run_PrintsAbsolutePathContent(t *testing.T) {
	t.Parallel()

	// Arrange
	tree := newFakeProjectTree("/repo", "/repo")
	streamer := newFakeFileStreamer()
	streamer.files["/repo/main.go"] = "package main\nfunc main() {}\n"
	output := &capturingTextOutput{}
	service := read.NewReadCommandService(tree, streamer, output)

	// Act
	err := service.Run("/repo/main.go")

	// Assert
	require.NoError(t, err)
	require.Len(t, output.lines, 2)
	assert.Equal(t, "package main", output.lines[0])
}

func TestReadCommandService_Run_ResolvesRelativePathFromCurrentDir(t *testing.T) {
	t.Parallel()

	// Arrange
	tree := newFakeProjectTree("/repo/cmd", "/repo")
	streamer := newFakeFileStreamer()
	streamer.files["/repo/cmd/main.go"] = pkgMainContent
	output := &capturingTextOutput{}
	service := read.NewReadCommandService(tree, streamer, output)

	// Act
	err := service.Run("main.go")

	// Assert
	require.NoError(t, err)
	require.Len(t, output.lines, 1)
	assert.Equal(t, "package main", output.lines[0])
}

func TestReadCommandService_Run_ResolvesRelativeSubdirectoryPath(t *testing.T) {
	t.Parallel()

	// Arrange
	tree := newFakeProjectTree("/repo", "/repo")
	streamer := newFakeFileStreamer()
	streamer.files["/repo/internal/foo.go"] = pkgFooContent
	output := &capturingTextOutput{}
	service := read.NewReadCommandService(tree, streamer, output)

	// Act
	err := service.Run("internal/foo.go")

	// Assert
	require.NoError(t, err)
	require.Len(t, output.lines, 1)
	assert.Equal(t, "package foo", output.lines[0])
}

func TestReadCommandService_Run_NormalizesDoubleDotPath(t *testing.T) {
	t.Parallel()

	// Arrange
	tree := newFakeProjectTree("/repo/cmd/idx", "/repo")
	streamer := newFakeFileStreamer()
	streamer.files["/repo/cmd/main.go"] = pkgMainContent
	output := &capturingTextOutput{}
	service := read.NewReadCommandService(tree, streamer, output)

	// Act
	err := service.Run("../main.go")

	// Assert
	require.NoError(t, err)
	require.Len(t, output.lines, 1)
	assert.Equal(t, "package main", output.lines[0])
}

// ---------------------------------------------------------------------------
// Directory detection
// ---------------------------------------------------------------------------

func TestReadCommandService_Run_ReturnsErrorForDirectoryPath(t *testing.T) {
	t.Parallel()

	// Arrange
	tree := newFakeProjectTree("/repo", "/repo")
	streamer := newFakeFileStreamer()
	streamer.dirs["/repo/internal"] = true
	output := &capturingTextOutput{}
	service := read.NewReadCommandService(tree, streamer, output)

	// Act
	err := service.Run("/repo/internal")

	// Assert
	require.Error(t, err)
	assert.ErrorContains(t, err, "directory")
	assert.Empty(t, output.lines)
}

// ---------------------------------------------------------------------------
// Project root enforcement
// ---------------------------------------------------------------------------

func TestReadCommandService_Run_RejectsPathOutsideProjectRoot(t *testing.T) {
	t.Parallel()

	// Arrange
	tree := newFakeProjectTree("/repo", "/repo")
	streamer := newFakeFileStreamer()
	streamer.files["/etc/passwd"] = "root:x:0:0\n"
	output := &capturingTextOutput{}
	service := read.NewReadCommandService(tree, streamer, output)

	// Act
	err := service.Run("/etc/passwd")

	// Assert
	require.Error(t, err)
	assert.ErrorContains(t, err, "outside project root")
	assert.Empty(t, output.lines)
}

func TestReadCommandService_Run_RejectsRelativePathEscapingRoot(t *testing.T) {
	t.Parallel()

	// Arrange
	tree := newFakeProjectTree("/repo/cmd", "/repo")
	streamer := newFakeFileStreamer()
	streamer.files["/etc/passwd"] = "root\n"
	output := &capturingTextOutput{}
	service := read.NewReadCommandService(tree, streamer, output)

	// Act
	err := service.Run("../../etc/passwd")

	// Assert
	require.Error(t, err)
	assert.ErrorContains(t, err, "outside project root")
}

// ---------------------------------------------------------------------------
// Line range — --from / --to
// ---------------------------------------------------------------------------

func TestReadCommandService_RunWithOptions_LineRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		fileContent   string
		from, to      int
		expectedLines []string
	}{
		{
			name:        "FromLine_PrintsFromGivenLine",
			fileContent: "line1\nline2\nline3\nline4\n",
			from:        3, to: 0,
			expectedLines: []string{"line3", "line4"},
		},
		{
			name:        "ToLine_PrintsUpToGivenLine",
			fileContent: "line1\nline2\nline3\nline4\n",
			from:        0, to: 2,
			expectedLines: []string{"line1", "line2"},
		},
		{
			name:        "BothBounds_PrintsRange",
			fileContent: "line1\nline2\nline3\nline4\nline5\n",
			from:        2, to: 4,
			expectedLines: []string{"line2", "line3", "line4"},
		},
		{
			name:        "FromBeyondEOF_PrintsNothing",
			fileContent: "line1\nline2\n",
			from:        99, to: 0,
			expectedLines: nil,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			tree := newFakeProjectTree("/repo", "/repo")
			streamer := newFakeFileStreamer()
			streamer.files["/repo/file.txt"] = tc.fileContent
			output := &capturingTextOutput{}
			service := read.NewReadCommandService(tree, streamer, output)

			// Act
			err := service.RunWithOptions("/repo/file.txt", tc.from, tc.to)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tc.expectedLines, output.lines)
		})
	}
}

// ---------------------------------------------------------------------------
// Error cases
// ---------------------------------------------------------------------------

func TestReadCommandService_Run_ReturnsErrorForMissingFile(t *testing.T) {
	t.Parallel()

	// Arrange
	tree := newFakeProjectTree("/repo", "/repo")
	streamer := newFakeFileStreamer()
	output := &capturingTextOutput{}
	service := read.NewReadCommandService(tree, streamer, output)

	// Act
	err := service.Run("/repo/missing.go")

	// Assert
	require.Error(t, err)
	assert.Empty(t, output.lines)
}

func TestReadCommandService_Run_ReturnsErrorWhenCurrentDirFails(t *testing.T) {
	t.Parallel()

	// Arrange
	tree := newFakeProjectTree("", "/repo")
	service := read.NewReadCommandService(tree, newFakeFileStreamer(), &capturingTextOutput{})

	// Act & Assert
	require.Error(t, service.Run("main.go"))
}

func TestReadCommandService_Run_NilDependencies_ReturnsError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		runFn func(string) error
	}{
		{
			name:  "AllNil",
			runFn: read.NewReadCommandService(nil, nil, nil).Run,
		},
		{
			name:  "NilProjectTree",
			runFn: read.NewReadCommandService(nil, newFakeFileStreamer(), &capturingTextOutput{}).Run,
		},
		{
			name:  "NilStreamer",
			runFn: read.NewReadCommandService(newFakeProjectTree("/repo", "/repo"), nil, &capturingTextOutput{}).Run,
		},
		{
			name:  "NilOutput",
			runFn: read.NewReadCommandService(newFakeProjectTree("/repo", "/repo"), newFakeFileStreamer(), nil).Run,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			require.Error(t, tc.runFn("main.go"))
		})
	}
}

func TestReadCommandService_Run_ErrorIncludesFilePath(t *testing.T) {
	t.Parallel()

	// Arrange
	tree := newFakeProjectTree("/repo", "/repo")
	service := read.NewReadCommandService(tree, newFakeFileStreamer(), &capturingTextOutput{})

	// Act
	err := service.Run("/repo/missing.go")

	// Assert
	require.Error(t, err)
	assert.ErrorContains(t, err, "missing.go")
}

// ---------------------------------------------------------------------------
// Read log integration
// ---------------------------------------------------------------------------

func TestReadCommandService_Run_RecordsAccessLogAfterSuccessfulRead(t *testing.T) {
	t.Parallel()

	// Arrange
	tree := newFakeProjectTree("/repo", "/repo")
	streamer := newFakeFileStreamer()
	streamer.files["/repo/main.go"] = pkgMainContent
	logRepo := &capturingReadLogRepository{}
	service := read.NewReadCommandService(tree, streamer, &capturingTextOutput{}).WithReadLog(logRepo)

	// Act
	require.NoError(t, service.Run("/repo/main.go"))

	// Assert
	require.Len(t, logRepo.calls, 1)
	assert.Equal(t, "main.go", logRepo.calls[0].relativePath)
	assert.Equal(t, "/repo", logRepo.calls[0].projectRoot)
}

func TestReadCommandService_Run_DoesNotRecordLogOnReadError(t *testing.T) {
	t.Parallel()

	// Arrange
	logRepo := &capturingReadLogRepository{}
	service := read.NewReadCommandService(
		newFakeProjectTree("/repo", "/repo"),
		newFakeFileStreamer(), // missing file → read fails
		&capturingTextOutput{},
	).WithReadLog(logRepo)

	// Act
	_ = service.Run("/repo/missing.go")

	// Assert
	assert.Empty(t, logRepo.calls)
}

func TestReadCommandService_Run_LogFailureDoesNotFailRead(t *testing.T) {
	t.Parallel()

	// Arrange
	tree := newFakeProjectTree("/repo", "/repo")
	streamer := newFakeFileStreamer()
	streamer.files["/repo/main.go"] = pkgMainContent
	output := &capturingTextOutput{}
	logRepo := &capturingReadLogRepository{err: errors.New("disk full")}
	service := read.NewReadCommandService(tree, streamer, output).WithReadLog(logRepo)

	// Act
	err := service.Run("/repo/main.go")

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, output.lines)
}

func TestReadCommandService_Run_DoesNotLogGitDirectoryFiles(t *testing.T) {
	t.Parallel()

	// Arrange
	tree := newFakeProjectTree("/repo", "/repo")
	streamer := newFakeFileStreamer()
	streamer.files["/repo/.git/config"] = "[core]\n"
	logRepo := &capturingReadLogRepository{}
	service := read.NewReadCommandService(tree, streamer, &capturingTextOutput{}).WithReadLog(logRepo)

	// Act
	require.NoError(t, service.Run("/repo/.git/config"))

	// Assert
	assert.Empty(t, logRepo.calls)
}

func TestReadCommandService_Run_DoesNotLogIdxDirectoryFiles(t *testing.T) {
	t.Parallel()

	// Arrange
	tree := newFakeProjectTree("/repo", "/repo")
	streamer := newFakeFileStreamer()
	streamer.files["/repo/.idx/read_log.idx"] = "2026-01-01T00:00:00;main.go;1;0\n"
	logRepo := &capturingReadLogRepository{}
	service := read.NewReadCommandService(tree, streamer, &capturingTextOutput{}).WithReadLog(logRepo)

	// Act
	require.NoError(t, service.Run("/repo/.idx/read_log.idx"))

	// Assert
	assert.Empty(t, logRepo.calls)
}

func TestReadCommandService_Run_RecordsRelativePathForSubdirectoryFile(t *testing.T) {
	t.Parallel()

	// Arrange
	tree := newFakeProjectTree("/repo", "/repo")
	streamer := newFakeFileStreamer()
	streamer.files["/repo/internal/foo.go"] = pkgFooContent
	logRepo := &capturingReadLogRepository{}
	service := read.NewReadCommandService(tree, streamer, &capturingTextOutput{}).WithReadLog(logRepo)

	// Act
	require.NoError(t, service.Run("/repo/internal/foo.go"))

	// Assert
	require.Len(t, logRepo.calls, 1)
	assert.Equal(t, "internal/foo.go", logRepo.calls[0].relativePath)
}

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

type fakeProjectTree struct {
	currentDir string
	gitRoot    string
	gitRootErr error
}

func newFakeProjectTree(currentDir, gitRoot string) *fakeProjectTree {
	return &fakeProjectTree{currentDir: currentDir, gitRoot: gitRoot}
}

func (t *fakeProjectTree) CurrentDir() (string, error) {
	if t.currentDir == "" {
		return "", errors.New("cwd unavailable")
	}
	return t.currentDir, nil
}

func (t *fakeProjectTree) FindGitRoot(_ string) (string, error) {
	if t.gitRootErr != nil {
		return "", t.gitRootErr
	}
	return t.gitRoot, nil
}

func (t *fakeProjectTree) ReadDir(_ string) ([]filesystem.DirectoryEntry, error) { return nil, nil }
func (t *fakeProjectTree) Exists(_ string) (bool, error)                         { return false, nil }
func (t *fakeProjectTree) RemoveAll(_ string) error                              { return nil }
func (t *fakeProjectTree) WriteFile(_ string, _ []byte) error                    { return nil }

type fakeFileStreamer struct {
	files map[string]string
	dirs  map[string]bool
}

func newFakeFileStreamer() *fakeFileStreamer {
	return &fakeFileStreamer{files: map[string]string{}, dirs: map[string]bool{}}
}

func (f *fakeFileStreamer) OpenFile(path string) (io.ReadCloser, error) {
	content, ok := f.files[filepath.Clean(path)]
	if !ok {
		return nil, errors.New("file not found: " + path)
	}
	return io.NopCloser(strings.NewReader(content)), nil
}

func (f *fakeFileStreamer) IsDir(path string) (bool, error) {
	return f.dirs[filepath.Clean(path)], nil
}

type capturingTextOutput struct {
	lines []string
}

func (o *capturingTextOutput) WriteLine(text string) error {
	o.lines = append(o.lines, text)
	return nil
}

type readLogCall struct {
	projectRoot  string
	relativePath string
}

type capturingReadLogRepository struct {
	calls []readLogCall
	err   error
}

func (r *capturingReadLogRepository) RecordRead(projectRoot, relativePath string) error {
	r.calls = append(r.calls, readLogCall{projectRoot: projectRoot, relativePath: relativePath})
	return r.err
}

func (r *capturingReadLogRepository) LoadAll(_ string) ([]read.LogEntry, error) {
	return nil, nil
}
