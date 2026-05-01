package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
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

	logger, err := newLogger()
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

	_, err := newLogger()
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
