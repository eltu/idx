package indexing_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"idx/internal/adapters/repository"
	"idx/internal/core/services/indexing"
)

func TestStatusReportsIndicesUpToDateWhenLatestLogsMatchFileTimestamp(t *testing.T) {
	rootDir := t.TempDir()
	ensureGitProject(t, rootDir)

	rootFile := filepath.Join(rootDir, "root.txt")
	nestedDir := filepath.Join(rootDir, "internal")
	nestedFile := filepath.Join(nestedDir, "service.txt")
	writeFile(t, rootFile, "root v1")
	writeFile(t, nestedFile, "service v1")

	output := &capturingTextOutput{}
	service := newStatusService(output)
	changeTo(t, rootDir)

	if err := service.Run(); err != nil {
		t.Fatalf("expected init to succeed, got %v", err)
	}

	alignIndexedFileMtimeWithLatestLog(t, rootDir)

	if err := service.Status(); err != nil {
		t.Fatalf("expected status to succeed, got %v", err)
	}

	if len(output.lines) == 0 {
		t.Fatal("expected status output line, got none")
	}

	if output.lines[len(output.lines)-1] != "✅ Indices are up to date." {
		t.Fatalf("expected up-to-date message, got %q", output.lines[len(output.lines)-1])
	}
}

func TestStatusFailsWhenLatestLogTimestampDiffersFromFileTimestamp(t *testing.T) {
	rootDir := t.TempDir()
	ensureGitProject(t, rootDir)

	rootFile := filepath.Join(rootDir, "root.txt")
	writeFile(t, rootFile, "root v1")

	service := newStatusService(&capturingTextOutput{})
	changeTo(t, rootDir)

	if err := service.Run(); err != nil {
		t.Fatalf("expected init to succeed, got %v", err)
	}

	logPath := filepath.Join(rootDir, ".idx", "logs", "tlog.idx")
	entry := latestLogEntry(t, logPath)

	if err := os.Chtimes(entry.path, entry.indexedAt.Add(5*time.Second), entry.indexedAt.Add(5*time.Second)); err != nil {
		t.Fatalf("expected file timestamp mutation to succeed, got %v", err)
	}

	err := service.Status()
	if err == nil {
		t.Fatal("expected status to fail when file timestamp differs from tlog indexed_at")
	}

	if !strings.Contains(err.Error(), "stale index record") {
		t.Fatalf("expected stale index record error, got %v", err)
	}
}

func newStatusService(output *capturingTextOutput) indexing.InitCommandService {
	return indexing.NewInitCommandService(
		repository.NewOSProjectTree(),
		fakeIgnoreMatcherFactory{ignoredPaths: map[string]bool{}},
		output,
		repository.NewOSFileReader(),
		indexing.NewBM25IndexService(),
		repository.NewBinaryIndexRepository(),
		repository.NewDirectoryChecksumRepository(),
		nil,
	)
}

func alignIndexedFileMtimeWithLatestLog(t *testing.T, projectRoot string) {
	t.Helper()

	projectTree := repository.NewOSProjectTree()
	directories, err := indexing.IndexedDirectories(projectTree, projectRoot)
	if err != nil {
		t.Fatalf("expected indexed directories lookup to succeed, got %v", err)
	}

	for _, directoryPath := range directories {
		logPath := filepath.Join(directoryPath, ".idx", "logs", "tlog.idx")
		entry := latestLogEntry(t, logPath)
		if err := os.Chtimes(entry.path, entry.indexedAt, entry.indexedAt); err != nil {
			t.Fatalf("expected file timestamp alignment to succeed for %q, got %v", entry.path, err)
		}
	}
}

type transactionLogEntry struct {
	path      string
	indexedAt time.Time
}

func latestLogEntry(t *testing.T, logPath string) transactionLogEntry {
	t.Helper()

	content, err := os.ReadFile(logPath) //nolint:gosec
	if err != nil {
		t.Fatalf("expected log file read to succeed, got %v", err)
	}

	line := latestNonEmptyLine(string(content))
	if line == "" {
		t.Fatalf("expected non-empty log line in %q", logPath)
	}

	pathValue := ""
	indexedAtValue := ""
	for _, part := range strings.Split(line, "\t") {
		if strings.HasPrefix(part, "path=") {
			pathValue = strings.TrimPrefix(part, "path=")
		}

		if strings.HasPrefix(part, "indexed_at=") {
			indexedAtValue = strings.TrimPrefix(part, "indexed_at=")
		}
	}

	if pathValue == "" || indexedAtValue == "" {
		t.Fatalf("expected path and indexed_at fields in %q", line)
	}

	indexedAt, err := time.Parse(time.RFC3339, indexedAtValue)
	if err != nil {
		t.Fatalf("expected RFC3339 indexed_at in %q, got %v", line, err)
	}

	return transactionLogEntry{path: pathValue, indexedAt: indexedAt}
}

func latestNonEmptyLine(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ""
	}

	lines := strings.Split(trimmed, "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

func ensureGitProject(t *testing.T, rootDir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(rootDir, ".git"), 0o750); err != nil {
		t.Fatalf("expected git marker creation to succeed, got %v", err)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("expected parent directory creation to succeed, got %v", err)
	}

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("expected file write to succeed, got %v", err)
	}
}

func changeTo(t *testing.T, directoryPath string) {
	t.Helper()
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("expected cwd read to succeed, got %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalDir) })

	if err := os.Chdir(directoryPath); err != nil {
		t.Fatalf("expected chdir to %q to succeed, got %v", directoryPath, err)
	}
}
