package read_test

import (
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"idx/internal/core/domain"
	"idx/internal/core/ports"
	read "idx/internal/core/services/read"
)

// ---------------------------------------------------------------------------
// Happy path — basic reads
// ---------------------------------------------------------------------------

func TestReadCommandServiceRunPrintsAbsolutePathContent(t *testing.T) {
	tree := newFakeProjectTree("/repo", "/repo")
	streamer := newFakeFileStreamer()
	streamer.files["/repo/main.go"] = "package main\nfunc main() {}\n"
	output := &capturingTextOutput{}

	service := read.NewReadCommandService(tree, streamer, output)
	if err := service.Run("/repo/main.go"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(output.lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(output.lines), output.lines)
	}
	if output.lines[0] != "package main" {
		t.Fatalf("unexpected first line %q", output.lines[0])
	}
}

func TestReadCommandServiceRunResolvesRelativePathFromCurrentDir(t *testing.T) {
	tree := newFakeProjectTree("/repo/cmd", "/repo")
	streamer := newFakeFileStreamer()
	streamer.files["/repo/cmd/main.go"] = "package main\n"
	output := &capturingTextOutput{}

	service := read.NewReadCommandService(tree, streamer, output)
	if err := service.Run("main.go"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(output.lines) != 1 || output.lines[0] != "package main" {
		t.Fatalf("unexpected output: %v", output.lines)
	}
}

func TestReadCommandServiceRunResolvesRelativeSubdirectoryPath(t *testing.T) {
	tree := newFakeProjectTree("/repo", "/repo")
	streamer := newFakeFileStreamer()
	streamer.files["/repo/internal/foo.go"] = "package foo\n"
	output := &capturingTextOutput{}

	service := read.NewReadCommandService(tree, streamer, output)
	if err := service.Run("internal/foo.go"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(output.lines) != 1 || output.lines[0] != "package foo" {
		t.Fatalf("unexpected output: %v", output.lines)
	}
}

func TestReadCommandServiceRunNormalizesDoubleDotPath(t *testing.T) {
	tree := newFakeProjectTree("/repo/cmd/idx", "/repo")
	streamer := newFakeFileStreamer()
	streamer.files["/repo/cmd/main.go"] = "package main\n"
	output := &capturingTextOutput{}

	service := read.NewReadCommandService(tree, streamer, output)
	if err := service.Run("../main.go"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(output.lines) != 1 || output.lines[0] != "package main" {
		t.Fatalf("unexpected output: %v", output.lines)
	}
}

// ---------------------------------------------------------------------------
// Directory detection
// ---------------------------------------------------------------------------

func TestReadCommandServiceRunReturnsErrorForDirectoryPath(t *testing.T) {
	tree := newFakeProjectTree("/repo", "/repo")
	streamer := newFakeFileStreamer()
	streamer.dirs["/repo/internal"] = true
	output := &capturingTextOutput{}

	service := read.NewReadCommandService(tree, streamer, output)
	err := service.Run("/repo/internal")
	if err == nil {
		t.Fatal("expected error for directory path, got nil")
	}

	if !strings.Contains(err.Error(), "directory") {
		t.Fatalf("expected 'directory' in error message, got %q", err.Error())
	}

	if len(output.lines) != 0 {
		t.Fatalf("expected no output on error, got %v", output.lines)
	}
}

// ---------------------------------------------------------------------------
// Project root enforcement
// ---------------------------------------------------------------------------

func TestReadCommandServiceRunRejectsPathOutsideProjectRoot(t *testing.T) {
	tree := newFakeProjectTree("/repo", "/repo")
	streamer := newFakeFileStreamer()
	streamer.files["/etc/passwd"] = "root:x:0:0\n"
	output := &capturingTextOutput{}

	service := read.NewReadCommandService(tree, streamer, output)
	err := service.Run("/etc/passwd")
	if err == nil {
		t.Fatal("expected error for path outside project root, got nil")
	}

	if !strings.Contains(err.Error(), "outside project root") {
		t.Fatalf("expected 'outside project root' in error, got %q", err.Error())
	}

	if len(output.lines) != 0 {
		t.Fatalf("expected no output on error, got %v", output.lines)
	}
}

func TestReadCommandServiceRunRejectsRelativePathEscapingRoot(t *testing.T) {
	tree := newFakeProjectTree("/repo/cmd", "/repo")
	streamer := newFakeFileStreamer()
	streamer.files["/etc/passwd"] = "root\n"
	output := &capturingTextOutput{}

	service := read.NewReadCommandService(tree, streamer, output)
	err := service.Run("../../etc/passwd")
	if err == nil {
		t.Fatal("expected error for path escaping project root, got nil")
	}

	if !strings.Contains(err.Error(), "outside project root") {
		t.Fatalf("expected 'outside project root' in error, got %q", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Line range — --from / --to
// ---------------------------------------------------------------------------

func TestReadCommandServiceRunWithOptionsFromLinePrintsFromGivenLine(t *testing.T) {
	tree := newFakeProjectTree("/repo", "/repo")
	streamer := newFakeFileStreamer()
	streamer.files["/repo/file.txt"] = "line1\nline2\nline3\nline4\n"
	output := &capturingTextOutput{}

	service := read.NewReadCommandService(tree, streamer, output)
	if err := service.RunWithOptions("/repo/file.txt", 3, 0); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(output.lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(output.lines), output.lines)
	}
	if output.lines[0] != "line3" {
		t.Fatalf("expected first line to be 'line3', got %q", output.lines[0])
	}
}

func TestReadCommandServiceRunWithOptionsToLinePrintsUpToGivenLine(t *testing.T) {
	tree := newFakeProjectTree("/repo", "/repo")
	streamer := newFakeFileStreamer()
	streamer.files["/repo/file.txt"] = "line1\nline2\nline3\nline4\n"
	output := &capturingTextOutput{}

	service := read.NewReadCommandService(tree, streamer, output)
	if err := service.RunWithOptions("/repo/file.txt", 0, 2); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(output.lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(output.lines), output.lines)
	}
	if output.lines[1] != "line2" {
		t.Fatalf("expected last line to be 'line2', got %q", output.lines[1])
	}
}

func TestReadCommandServiceRunWithOptionsBothBoundsPrintsRange(t *testing.T) {
	tree := newFakeProjectTree("/repo", "/repo")
	streamer := newFakeFileStreamer()
	streamer.files["/repo/file.txt"] = "line1\nline2\nline3\nline4\nline5\n"
	output := &capturingTextOutput{}

	service := read.NewReadCommandService(tree, streamer, output)
	if err := service.RunWithOptions("/repo/file.txt", 2, 4); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(output.lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %v", len(output.lines), output.lines)
	}
	if output.lines[0] != "line2" || output.lines[2] != "line4" {
		t.Fatalf("unexpected range output: %v", output.lines)
	}
}

func TestReadCommandServiceRunWithOptionsFromBeyondEOFPrintsNothing(t *testing.T) {
	tree := newFakeProjectTree("/repo", "/repo")
	streamer := newFakeFileStreamer()
	streamer.files["/repo/file.txt"] = "line1\nline2\n"
	output := &capturingTextOutput{}

	service := read.NewReadCommandService(tree, streamer, output)
	if err := service.RunWithOptions("/repo/file.txt", 99, 0); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(output.lines) != 0 {
		t.Fatalf("expected no output when from exceeds file length, got %v", output.lines)
	}
}

// ---------------------------------------------------------------------------
// Error cases
// ---------------------------------------------------------------------------

func TestReadCommandServiceRunReturnsErrorForMissingFile(t *testing.T) {
	tree := newFakeProjectTree("/repo", "/repo")
	streamer := newFakeFileStreamer()
	output := &capturingTextOutput{}

	service := read.NewReadCommandService(tree, streamer, output)
	err := service.Run("/repo/missing.go")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}

	if len(output.lines) != 0 {
		t.Fatalf("expected no output on error, got %v", output.lines)
	}
}

func TestReadCommandServiceRunReturnsErrorWhenCurrentDirFails(t *testing.T) {
	tree := newFakeProjectTree("", "/repo")
	streamer := newFakeFileStreamer()
	output := &capturingTextOutput{}

	service := read.NewReadCommandService(tree, streamer, output)
	if err := service.Run("main.go"); err == nil {
		t.Fatal("expected error when current directory resolution fails")
	}
}

func TestReadCommandServiceRunReturnsErrorForNilDependencies(t *testing.T) {
	service := read.NewReadCommandService(nil, nil, nil)
	if err := service.Run("main.go"); err == nil {
		t.Fatal("expected dependency validation error, got nil")
	}
}

func TestReadCommandServiceRunReturnsErrorForNilProjectTree(t *testing.T) {
	service := read.NewReadCommandService(nil, newFakeFileStreamer(), &capturingTextOutput{})
	if err := service.Run("main.go"); err == nil {
		t.Fatal("expected nil projectTree error")
	}
}

func TestReadCommandServiceRunReturnsErrorForNilStreamer(t *testing.T) {
	service := read.NewReadCommandService(newFakeProjectTree("/repo", "/repo"), nil, &capturingTextOutput{})
	if err := service.Run("main.go"); err == nil {
		t.Fatal("expected nil streamer error")
	}
}

func TestReadCommandServiceRunReturnsErrorForNilOutput(t *testing.T) {
	service := read.NewReadCommandService(newFakeProjectTree("/repo", "/repo"), newFakeFileStreamer(), nil)
	if err := service.Run("main.go"); err == nil {
		t.Fatal("expected nil output error")
	}
}

func TestReadCommandServiceRunErrorIncludesFilePath(t *testing.T) {
	tree := newFakeProjectTree("/repo", "/repo")
	streamer := newFakeFileStreamer()
	output := &capturingTextOutput{}

	service := read.NewReadCommandService(tree, streamer, output)
	err := service.Run("/repo/missing.go")
	if err == nil {
		t.Fatal("expected error for missing file")
	}

	if !strings.Contains(err.Error(), "missing.go") {
		t.Fatalf("expected error to mention file path, got %q", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Read log integration
// ---------------------------------------------------------------------------

func TestReadCommandServiceRunRecordsAccessLogAfterSuccessfulRead(t *testing.T) {
	tree := newFakeProjectTree("/repo", "/repo")
	streamer := newFakeFileStreamer()
	streamer.files["/repo/main.go"] = "package main\n"
	output := &capturingTextOutput{}
	logRepo := &capturingReadLogRepository{}

	service := read.NewReadCommandService(tree, streamer, output).WithReadLog(logRepo)
	if err := service.Run("/repo/main.go"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(logRepo.calls) != 1 {
		t.Fatalf("expected 1 log call, got %d", len(logRepo.calls))
	}
	if logRepo.calls[0].relativePath != "main.go" {
		t.Fatalf("expected logged path 'main.go', got %q", logRepo.calls[0].relativePath)
	}
	if logRepo.calls[0].projectRoot != "/repo" {
		t.Fatalf("expected logged root '/repo', got %q", logRepo.calls[0].projectRoot)
	}
}

func TestReadCommandServiceRunDoesNotRecordLogOnReadError(t *testing.T) {
	tree := newFakeProjectTree("/repo", "/repo")
	streamer := newFakeFileStreamer() // missing file → read fails
	output := &capturingTextOutput{}
	logRepo := &capturingReadLogRepository{}

	service := read.NewReadCommandService(tree, streamer, output).WithReadLog(logRepo)
	_ = service.Run("/repo/missing.go")

	if len(logRepo.calls) != 0 {
		t.Fatalf("expected no log call on read failure, got %d", len(logRepo.calls))
	}
}

func TestReadCommandServiceRunLogFailureDoesNotFailRead(t *testing.T) {
	tree := newFakeProjectTree("/repo", "/repo")
	streamer := newFakeFileStreamer()
	streamer.files["/repo/main.go"] = "package main\n"
	output := &capturingTextOutput{}
	logRepo := &capturingReadLogRepository{err: errors.New("disk full")}

	service := read.NewReadCommandService(tree, streamer, output).WithReadLog(logRepo)
	if err := service.Run("/repo/main.go"); err != nil {
		t.Fatalf("expected read to succeed even when log fails, got %v", err)
	}

	if len(output.lines) == 0 {
		t.Fatal("expected file content to be printed despite log error")
	}
}

func TestReadCommandServiceRunDoesNotLogGitDirectoryFiles(t *testing.T) {
	tree := newFakeProjectTree("/repo", "/repo")
	streamer := newFakeFileStreamer()
	streamer.files["/repo/.git/config"] = "[core]\n"
	output := &capturingTextOutput{}
	logRepo := &capturingReadLogRepository{}

	service := read.NewReadCommandService(tree, streamer, output).WithReadLog(logRepo)
	if err := service.Run("/repo/.git/config"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(logRepo.calls) != 0 {
		t.Fatalf("expected no log call for .git file, got %d", len(logRepo.calls))
	}
}

func TestReadCommandServiceRunDoesNotLogIdxDirectoryFiles(t *testing.T) {
	tree := newFakeProjectTree("/repo", "/repo")
	streamer := newFakeFileStreamer()
	streamer.files["/repo/.idx/read_log.idx"] = "2026-01-01T00:00:00;main.go;1;0\n"
	output := &capturingTextOutput{}
	logRepo := &capturingReadLogRepository{}

	service := read.NewReadCommandService(tree, streamer, output).WithReadLog(logRepo)
	if err := service.Run("/repo/.idx/read_log.idx"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(logRepo.calls) != 0 {
		t.Fatalf("expected no log call for .idx file, got %d", len(logRepo.calls))
	}
}

func TestReadCommandServiceRunRecordsRelativePathForSubdirectoryFile(t *testing.T) {
	tree := newFakeProjectTree("/repo", "/repo")
	streamer := newFakeFileStreamer()
	streamer.files["/repo/internal/foo.go"] = "package foo\n"
	output := &capturingTextOutput{}
	logRepo := &capturingReadLogRepository{}

	service := read.NewReadCommandService(tree, streamer, output).WithReadLog(logRepo)
	if err := service.Run("/repo/internal/foo.go"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(logRepo.calls) != 1 {
		t.Fatalf("expected 1 log call, got %d", len(logRepo.calls))
	}
	if logRepo.calls[0].relativePath != "internal/foo.go" {
		t.Fatalf("expected relative path 'internal/foo.go', got %q", logRepo.calls[0].relativePath)
	}
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

func (t *fakeProjectTree) ReadDir(_ string) ([]domain.DirectoryEntry, error) { return nil, nil }
func (t *fakeProjectTree) Exists(_ string) (bool, error)                     { return false, nil }
func (t *fakeProjectTree) RemoveAll(_ string) error                          { return nil }
func (t *fakeProjectTree) WriteFile(_ string, _ []byte) error                { return nil }

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

func (r *capturingReadLogRepository) LoadAll(_ string) ([]ports.ReadLogEntry, error) {
	return nil, nil
}
