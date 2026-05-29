package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appcli "idx/internal/app/cli"
)

func TestRunReturnsErrorForUnsupportedCommand(t *testing.T) {
	t.Parallel()
	err := run([]string{"idx", "unknown-command"}, &bytes.Buffer{})
	require.Error(t, err)
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
		require.NotNil(t, recovered, "expected panic from exit hook")
		assert.True(t, exitCalled, "expected exit hook to be called")
		assert.Equal(t, 1, exitCode)
	}()

	main()
}

func TestProjectNameFromDir_GitRoot_UsesBaseName(t *testing.T) {
	t.Parallel()

	// Arrange
	workspace := filepath.Join(t.TempDir(), "my-project")
	nested := filepath.Join(workspace, "internal", "core")
	require.NoError(t, os.MkdirAll(filepath.Join(workspace, ".git"), 0o750))
	require.NoError(t, os.MkdirAll(nested, 0o750))

	// Act
	name := projectNameFromDir(nested)

	// Assert
	assert.Equal(t, "my-project", name)
}

func TestSanitizePathSegment_UnsupportedChars_ReplacedWithUnderscore(t *testing.T) {
	t.Parallel()
	result := sanitizePathSegment("my project@2026")
	assert.Equal(t, "my_project_2026", result)
}

func TestSanitizePathSegment_AllDots_FallsBackToUnknown(t *testing.T) {
	t.Parallel()
	result := sanitizePathSegment("...")
	assert.Equal(t, "unknown-project", result)
}

func TestSanitizePathSegment_SingleDot_FallsBackToUnknown(t *testing.T) {
	t.Parallel()
	result := sanitizePathSegment(".")
	assert.Equal(t, "unknown-project", result)
}

func TestNewLogger_ValidIDXLogLevel_ReturnsLogger(t *testing.T) {
	// Arrange
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("IDX_LOG_LEVEL", "debug")

	// Act
	logger, err := newLogger("")

	// Assert
	require.NoError(t, err)
	require.NotNil(t, logger)
	defer logger.Sync() //nolint:errcheck
}

func TestNewLogger_InvalidIDXLogLevel_ReturnsError(t *testing.T) {
	// Arrange
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("IDX_LOG_LEVEL", "not-a-level")

	// Act + Assert
	_, err := newLogger("")
	require.Error(t, err)
}

func TestLoggerOutputPath_CreatesDir_ReturnsLogPath(t *testing.T) {
	// Arrange
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Act
	path, err := loggerOutputPath()

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, path)
	assert.Equal(t, ".log", filepath.Ext(path))
}

func TestIsServerCommand_ReturnsTrue_ForServerRun(t *testing.T) {
	t.Parallel()
	assert.True(t, isServerCommand([]string{"idx", "server", "run"}))
}

func TestIsServerCommand_ReturnsFalse_ForVariousNonRunArgs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		args []string
	}{
		{"server start", []string{"idx", "server", "start"}},
		{"server stop", []string{"idx", "server", "stop"}},
		{"server status", []string{"idx", "server", "status"}},
		{"server only", []string{"idx", "server"}},
		{"non-server subcommand", []string{"idx", "search", "query"}},
		{"flags only", []string{"idx", "--quiet"}},
		{"empty args", []string{"idx"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.False(t, isServerCommand(tc.args))
		})
	}
}

func TestIsServerCommand_IgnoresFlagsBeforeServerRun(t *testing.T) {
	t.Parallel()
	assert.True(t, isServerCommand([]string{"idx", "--quiet", "server", "run"}))
}

func TestMultiQuiet_SetQuietTrue_SuppressesOutput(t *testing.T) {
	t.Parallel()

	// Arrange
	var buf bytes.Buffer
	wa := appcli.NewLineWriter(&buf)
	wb := appcli.NewLineWriter(&buf)
	mq := multiQuiet{wa, wb}

	// Act
	mq.SetQuiet(true)
	err := wa.WriteLine("suppressed")

	// Assert
	require.NoError(t, err)
	assert.Empty(t, buf.String(), "expected no output after SetQuiet(true)")
}

func TestEarlyLoadConfigLogLevel_NoConfigFile_ReturnsDefault(t *testing.T) {
	t.Parallel()
	level := earlyLoadConfigLogLevel(t.TempDir())
	// No config file → defaults are returned; default log level is "error"
	assert.NotEmpty(t, level)
}

func TestEarlyLoadConfigLogLevel_ConfigFile_ReturnsFileLevel(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".idx.yml")
	require.NoError(t, os.WriteFile(configPath, []byte("log:\n  level: debug\n"), 0o600))

	// Act
	level := earlyLoadConfigLogLevel(dir)

	// Assert
	assert.Equal(t, "debug", level)
}

func TestEarlyLoadConfigLogLevel_MalformedYAML_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".idx.yml")
	require.NoError(t, os.WriteFile(configPath, []byte("not: valid: yaml: {{{"), 0o600))

	// Act
	level := earlyLoadConfigLogLevel(dir)

	// Assert
	assert.Empty(t, level)
}

func TestGitRootFrom_NoGitDir_FallsBackToStartDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	got := gitRootFrom(dir)
	assert.Equal(t, dir, got)
}

func TestIsPathSafeChar_AcceptsAlphanumericAndDash(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		ch   byte
		want bool
	}{
		{"uppercase A", 'A', true},
		{"uppercase Z", 'Z', true},
		{"lowercase a", 'a', true},
		{"lowercase z", 'z', true},
		{"digit 0", '0', true},
		{"digit 9", '9', true},
		{"space", ' ', false},
		{"at sign", '@', false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, isPathSafeChar(tc.ch))
		})
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
		assert.Equal(t, 1, exitCode)
	}()

	main()
}
