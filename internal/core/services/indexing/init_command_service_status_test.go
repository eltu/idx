package indexing_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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

	if err := service.Status(); err != nil {
		t.Fatalf("expected status to succeed, got %v", err)
	}

	if len(output.lines) == 0 {
		t.Fatal("expected status output line, got none")
	}

	if output.lines[len(output.lines)-1] != "\n✅ Indices are up to date.\n" {
		t.Fatalf("expected up-to-date message, got %q", output.lines[len(output.lines)-1])
	}
}

func TestStatusWithProfileReportsDetailedTableAndSummary(t *testing.T) {
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

	if err := service.StatusWithProfile(true); err != nil {
		t.Fatalf("expected profile status to succeed, got %v", err)
	}

	outputText := strings.Join(output.lines, "\n")
	if !strings.Contains(outputText, "📊 Summary") {
		t.Fatalf("expected summary header in output, got %q", outputText)
	}

	if !strings.Contains(outputText, "files checked") {
		t.Fatalf("expected files checked row in output, got %q", outputText)
	}

	if !strings.Contains(outputText, "root.txt") {
		t.Fatalf("expected checked row for root file in output, got %q", outputText)
	}

	if !strings.Contains(outputText, "✓") {
		t.Fatalf("expected updated marker in output, got %q", outputText)
	}
}

func TestStatusFailsWhenFileChangedAfterLastIndex(t *testing.T) {
	rootDir := t.TempDir()
	ensureGitProject(t, rootDir)

	rootFile := filepath.Join(rootDir, "root.txt")
	writeFile(t, rootFile, "root v1")

	output := &capturingTextOutput{}
	service := newStatusService(output)
	changeTo(t, rootDir)

	if err := service.Run(); err != nil {
		t.Fatalf("expected init to succeed, got %v", err)
	}
	linesBeforeStatus := len(output.lines)

	writeFile(t, rootFile, "root v2 — changed after indexing")

	err := service.Status()
	if err == nil {
		t.Fatal("expected status to fail when file content changed after indexing")
	}

	if !strings.Contains(err.Error(), "stale index") {
		t.Fatalf("expected stale index error, got %v", err)
	}

	if len(output.lines) != linesBeforeStatus {
		t.Fatalf("expected simple status to avoid profile output, got %q", strings.Join(output.lines, "\n"))
	}
}

func TestStatusFailsWhenNewDirectoryIsNotIndexed(t *testing.T) {
	rootDir := t.TempDir()
	ensureGitProject(t, rootDir)

	rootFile := filepath.Join(rootDir, "root.txt")
	writeFile(t, rootFile, "root v1")

	service := newStatusService(&capturingTextOutput{})
	changeTo(t, rootDir)

	if err := service.Run(); err != nil {
		t.Fatalf("expected init to succeed, got %v", err)
	}

	// Add a new subdirectory with a file after indexing — it should be detected as missing.
	newDir := filepath.Join(rootDir, "newpkg")
	writeFile(t, filepath.Join(newDir, "handler.txt"), "new content")

	err := service.Status()
	if err == nil {
		t.Fatal("expected status to fail when a new unindexed directory exists")
	}

	if !strings.Contains(err.Error(), "unindexed") {
		t.Fatalf("expected unindexed directories error, got %v", err)
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
