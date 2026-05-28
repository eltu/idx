package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	appcli "idx/internal/app/cli"
)

func TestRunReturnsErrorForUnsupportedCommand(t *testing.T) {
	err := run([]string{"idx", "unknown-command"}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected unsupported command error, got nil")
	}
}

func TestMainCallsExitWithCodeOneWhenRunFails(t *testing.T) {
	originalArgs := os.Args
	originalExit := exitProcess
	os.Args = []string{"idx", "unknown-command"}
	t.Cleanup(func() {
		os.Args = originalArgs
		exitProcess = originalExit
	})

	exitCalled := false
	exitCode := 0
	exitProcess = func(code int) {
		exitCalled = true
		exitCode = code
		panic(errors.New("exit called"))
	}

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected panic from exit hook, got nil")
		}
		if !exitCalled {
			t.Fatal("expected exit hook to be called")
		}
		if exitCode != 1 {
			t.Fatalf("expected exit code 1, got %d", exitCode)
		}
	}()

	main()
}

func TestProjectNameFromDirUsesGitRootBaseName(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "my-project")
	nested := filepath.Join(workspace, "internal", "core")
	if err := os.MkdirAll(filepath.Join(workspace, ".git"), 0o750); err != nil {
		t.Fatalf("failed to create fake git root: %v", err)
	}
	if err := os.MkdirAll(nested, 0o750); err != nil {
		t.Fatalf("failed to create nested path: %v", err)
	}

	name := projectNameFromDir(nested)
	if name != "my-project" {
		t.Fatalf("expected project name %q, got %q", "my-project", name)
	}
}

func TestSanitizePathSegmentReplacesUnsupportedChars(t *testing.T) {
	result := sanitizePathSegment("my project@2026")
	if result != "my_project_2026" {
		t.Fatalf("expected sanitized name %q, got %q", "my_project_2026", result)
	}
}

func TestSanitizePathSegmentFallbackForEmptyName(t *testing.T) {
	result := sanitizePathSegment("...")
	if result != "unknown-project" {
		t.Fatalf("expected fallback name %q, got %q", "unknown-project", result)
	}
}

func TestNewLoggerWithValidIDXLogLevel(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("IDX_LOG_LEVEL", "debug")

	logger, err := newLogger("")
	if err != nil {
		t.Fatalf("expected no error with valid log level, got %v", err)
	}
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	defer logger.Sync() //nolint:errcheck
}

func TestNewLoggerWithInvalidIDXLogLevel(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("IDX_LOG_LEVEL", "not-a-level")

	_, err := newLogger("")
	if err == nil {
		t.Fatal("expected error with invalid log level, got nil")
	}
}

func TestLoggerOutputPathCreatesDirUnderHome(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	path, err := loggerOutputPath()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if path == "" {
		t.Fatal("expected non-empty log path")
	}
	if filepath.Ext(path) != ".log" {
		t.Fatalf("expected .log extension, got %q", path)
	}
}

func TestIsServerCommandReturnsTrueForServerRun(t *testing.T) {
	if !isServerCommand([]string{"idx", "server", "run"}) {
		t.Error("expected true for 'idx server run'")
	}
}

func TestIsServerCommandReturnsFalseForServerStart(t *testing.T) {
	if isServerCommand([]string{"idx", "server", "start"}) {
		t.Error("expected false for 'idx server start'")
	}
}

func TestIsServerCommandReturnsFalseForServerStop(t *testing.T) {
	if isServerCommand([]string{"idx", "server", "stop"}) {
		t.Error("expected false for 'idx server stop'")
	}
}

func TestIsServerCommandReturnsFalseForServerStatus(t *testing.T) {
	if isServerCommand([]string{"idx", "server", "status"}) {
		t.Error("expected false for 'idx server status'")
	}
}

func TestIsServerCommandReturnsFalseForServerOnly(t *testing.T) {
	if isServerCommand([]string{"idx", "server"}) {
		t.Error("expected false for 'idx server' alone")
	}
}

func TestIsServerCommandReturnsFalseForNonServerSubcommand(t *testing.T) {
	if isServerCommand([]string{"idx", "search", "query"}) {
		t.Error("expected false for 'idx search'")
	}
}

func TestIsServerCommandReturnsFalseForFlagsOnly(t *testing.T) {
	if isServerCommand([]string{"idx", "--quiet"}) {
		t.Error("expected false when only flags are present")
	}
}

func TestIsServerCommandReturnsFalseForEmptyArgs(t *testing.T) {
	if isServerCommand([]string{"idx"}) {
		t.Error("expected false for empty args")
	}
}

func TestIsServerCommandIgnoresFlagsBeforeServerRun(t *testing.T) {
	if !isServerCommand([]string{"idx", "--quiet", "server", "run"}) {
		t.Error("expected true when 'server run' follows flags")
	}
}

func TestMultiQuietSetQuietPropagates(t *testing.T) {
	var buf bytes.Buffer
	wa := appcli.NewLineWriter(&buf)
	wb := appcli.NewLineWriter(&buf)

	mq := multiQuiet{wa, wb}
	mq.SetQuiet(true)

	// After SetQuiet(true), writing to wa should be suppressed (no output to buf)
	if err := wa.WriteLine("suppressed"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no output after SetQuiet(true), got %q", buf.String())
	}
}

func TestEarlyLoadConfigLogLevelReturnsDefaultWhenFileAbsent(t *testing.T) {
	level := earlyLoadConfigLogLevel(t.TempDir())
	// No config file → defaults are returned; default log level is "error"
	if level == "" {
		t.Error("expected non-empty default log level, got empty string")
	}
}

func TestEarlyLoadConfigLogLevelReturnsLevelFromFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".idx.yml")
	if err := os.WriteFile(configPath, []byte("log:\n  level: debug\n"), 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	level := earlyLoadConfigLogLevel(dir)
	if level != "debug" {
		t.Errorf("expected %q, got %q", "debug", level)
	}
}

func TestGitRootFromFallsBackToStartDirWhenNoGitDir(t *testing.T) {
	dir := t.TempDir()
	got := gitRootFrom(dir)
	if got != dir {
		t.Errorf("expected fallback to start dir %q, got %q", dir, got)
	}
}

func TestIsPathSafeCharAcceptsUppercase(t *testing.T) {
	for ch := byte('A'); ch <= 'Z'; ch++ {
		if !isPathSafeChar(ch) {
			t.Errorf("expected uppercase %q to be safe", ch)
		}
	}
}

func TestIsPathSafeCharAcceptsDigits(t *testing.T) {
	for ch := byte('0'); ch <= '9'; ch++ {
		if !isPathSafeChar(ch) {
			t.Errorf("expected digit %q to be safe", ch)
		}
	}
}

func TestIsPathSafeCharRejectsSpace(t *testing.T) {
	if isPathSafeChar(' ') {
		t.Error("expected space to be unsafe")
	}
}

func TestSanitizePathSegmentFallbackForDotName(t *testing.T) {
	result := sanitizePathSegment(".")
	if result != "unknown-project" {
		t.Fatalf("expected 'unknown-project' for '.', got %q", result)
	}
}

func TestEarlyLoadConfigLogLevelReturnsEmptyForMalformedYAML(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".idx.yml")
	if err := os.WriteFile(configPath, []byte("not: valid: yaml: {{{"), 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	level := earlyLoadConfigLogLevel(dir)
	if level != "" {
		t.Errorf("expected empty level for malformed YAML, got %q", level)
	}
}

func TestMainContinuesWhenLoggerFails(t *testing.T) {
	tmpHome := t.TempDir()
	if err := os.Chmod(tmpHome, 0o555); err != nil {
		t.Skip("cannot set directory read-only, skipping")
	}
	defer func() { _ = os.Chmod(tmpHome, 0o750) }()

	originalArgs := os.Args
	originalExit := exitProcess
	os.Args = []string{"idx", "unknown-command"}
	t.Setenv("HOME", tmpHome)
	t.Cleanup(func() {
		os.Args = originalArgs
		exitProcess = originalExit
	})

	var exitCode int
	exitProcess = func(code int) {
		exitCode = code
		panic(errors.New("exit called"))
	}

	defer func() {
		recover() //nolint:errcheck
		if exitCode != 1 {
			t.Errorf("expected exit code 1, got %d", exitCode)
		}
	}()

	main()
}
