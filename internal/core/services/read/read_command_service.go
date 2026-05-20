package read

import (
	"bufio"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"idx/internal/core/ports"
)

// noopReadLog is the default when no log repository is injected.
type noopReadLog struct{}

func (noopReadLog) RecordRead(_, _ string) error                   { return nil }
func (noopReadLog) LoadAll(_ string) ([]ports.ReadLogEntry, error) { return nil, nil }

// fileStreamer abstracts opening a file for sequential reading and checking if a path is a directory.
type fileStreamer interface {
	OpenFile(path string) (io.ReadCloser, error)
	IsDir(path string) (bool, error)
}

type ReadCommandService struct {
	projectTree ports.ProjectTree
	streamer    fileStreamer
	output      ports.TextOutput
	logRepo     ports.ReadLogRepository
}

// NewReadCommandService builds the read use case.
// Example: service := NewReadCommandService(projectTree, streamer, output).
func NewReadCommandService(projectTree ports.ProjectTree, streamer fileStreamer, output ports.TextOutput) ReadCommandService {
	return ReadCommandService{
		projectTree: projectTree,
		streamer:    streamer,
		output:      output,
		logRepo:     noopReadLog{},
	}
}

// WithReadLog attaches a read log repository that records file access history.
// Example: service = service.WithReadLog(readLogRepo).
func (service ReadCommandService) WithReadLog(repo ports.ReadLogRepository) ReadCommandService {
	service.logRepo = repo
	return service
}

// Run prints the full content of filePath to output.
// Example: err := service.Run("internal/core/ports/text_output.go").
func (service ReadCommandService) Run(filePath string) error {
	return service.RunWithOptions(filePath, 0, 0)
}

// RunWithOptions prints the content of filePath restricted to lines [fromLine, toLine].
// fromLine and toLine are 1-based; zero means unbounded on that end.
// Example: err := service.RunWithOptions("main.go", 10, 20).
func (service ReadCommandService) RunWithOptions(filePath string, fromLine, toLine int) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}

	resolved, err := service.resolvePath(filePath)
	if err != nil {
		return err
	}

	projectRoot, err := service.findProjectRoot()
	if err != nil {
		return err
	}

	if err := enforceProjectBounds(resolved, projectRoot); err != nil {
		return err
	}

	isDir, err := service.streamer.IsDir(resolved)
	if err != nil {
		return fmt.Errorf("failed to stat %q: got error %v, expected a readable file path", filePath, err)
	}
	if isDir {
		return fmt.Errorf("cannot read %q: got a directory, expected a file path", filePath)
	}

	if err := service.streamLines(resolved, filePath, fromLine, toLine); err != nil {
		return err
	}

	service.recordReadAccess(projectRoot, resolved)
	return nil
}

// recordReadAccess logs the file access; errors are swallowed because the log is supplementary.
// Paths under .git or .idx are skipped — they are system directories, not project content.
func (service ReadCommandService) recordReadAccess(projectRoot, resolved string) {
	rel, err := filepath.Rel(projectRoot, resolved)
	if err != nil {
		return
	}
	if isSystemPath(rel) {
		return
	}
	_ = service.logRepo.RecordRead(projectRoot, filepath.ToSlash(rel))
}

func isSystemPath(rel string) bool {
	top := strings.SplitN(filepath.ToSlash(rel), "/", 2)[0]
	return top == ".git" || top == ".idx"
}

func (service ReadCommandService) streamLines(resolved, original string, fromLine, toLine int) error {
	reader, err := service.streamer.OpenFile(resolved)
	if err != nil {
		return fmt.Errorf("failed to read file %q: got error %v, expected a readable file", original, err)
	}
	defer reader.Close()

	scanner := bufio.NewScanner(reader)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		if fromLine > 0 && lineNum < fromLine {
			continue
		}
		if toLine > 0 && lineNum > toLine {
			break
		}
		if err := service.output.WriteLine(scanner.Text()); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func (service ReadCommandService) resolvePath(filePath string) (string, error) {
	if filepath.IsAbs(filePath) {
		return filepath.Clean(filePath), nil
	}

	currentDir, err := service.projectTree.CurrentDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve current directory: got error %v, expected a readable working directory", err)
	}

	return filepath.Clean(filepath.Join(currentDir, filePath)), nil
}

func (service ReadCommandService) findProjectRoot() (string, error) {
	currentDir, err := service.projectTree.CurrentDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve current directory: got error %v, expected a readable working directory", err)
	}
	return service.projectTree.FindGitRoot(currentDir)
}

// enforceProjectBounds returns an error when resolved is not under projectRoot.
func enforceProjectBounds(resolved, projectRoot string) error {
	rel, err := filepath.Rel(projectRoot, resolved)
	if err != nil || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("path %q is outside project root %q: expected a path within the project", resolved, projectRoot)
	}
	return nil
}

func (service ReadCommandService) validateDependencies() error {
	if service.projectTree == nil {
		return fmt.Errorf("failed to run read command: got nil projectTree dependency, expected non-nil ports.ProjectTree")
	}

	if service.streamer == nil {
		return fmt.Errorf("failed to run read command: got nil streamer dependency, expected non-nil fileStreamer")
	}

	if service.output == nil {
		return fmt.Errorf("failed to run read command: got nil output dependency, expected non-nil ports.TextOutput")
	}

	return nil
}
