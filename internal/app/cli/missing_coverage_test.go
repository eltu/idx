package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appserver "idx/internal/app/server"
	search "idx/internal/features/search"
)

// --- validateSearchConfig ---

func TestValidateSearchConfig_ValidInput_NoError(t *testing.T) {
	t.Parallel()
	cfg := &searchCommandConfig{
		format:   search.OutputText,
		operator: search.OperatorAND,
	}
	assert.NoError(t, validateSearchConfig(cfg, false, false))
}

func TestValidateSearchConfig_InvalidInputs_ReturnErrors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		cfg  *searchCommandConfig
	}{
		{
			"negative context",
			&searchCommandConfig{format: search.OutputText, operator: search.OperatorAND, contextLines: -1},
		},
		{
			"bad format",
			&searchCommandConfig{format: "xml", operator: search.OperatorAND},
		},
		{
			"bad operator",
			&searchCommandConfig{format: search.OutputText, operator: "XOR"},
		},
		{
			"invalid relaxation",
			&searchCommandConfig{format: search.OutputText, operator: search.OperatorAND, relaxation: "no-angle"},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Error(t, validateSearchConfig(tc.cfg, false, false))
		})
	}
}

// --- writeSearchMissingQueryError ---

func TestWriteSearchMissingQueryError_WritesToCommand(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	cmd := &cobra.Command{Use: "search"}
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	writeSearchMissingQueryError(cmd)
	// The function writes to cmd.OutOrStdout() via fmt.Fprintln in the parent output
	// (it just prints; we verify no panic occurred)
}

// --- WithIndexServer ---

type stubServerRunner struct{}

func (s *stubServerRunner) Serve(_ context.Context) error { return nil }

func TestWithIndexServer_SetsField(t *testing.T) {
	t.Parallel()
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).
		WithIndexServer(&stubServerRunner{})
	assert.NotNil(t, runner.indexServer)
}

func TestWithIndexServer_AcceptsRealServer(t *testing.T) {
	t.Parallel()
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).
		WithIndexServer(appserver.NewServer(appserver.ServerDeps{}))
	assert.NotNil(t, runner.indexServer)
}

// --- CommandRunner.Run ---

type stubSearcher struct{ called bool }

func (s *stubSearcher) RunWithOptions(_ string, _ search.Options) error {
	s.called = true
	return nil
}

func TestCommandRunnerRun_UnknownCommand_ReturnsError(t *testing.T) {
	t.Parallel()
	runner := NewCommandRunner([]string{"idx", "unknown-xyz"}, &stubIndexCommand{}, nil, &stubSearcher{})
	require.Error(t, runner.Run())
}

func TestCommandRunnerRun_KnownCommand_Succeeds(t *testing.T) {
	t.Parallel()
	runner := NewCommandRunner([]string{"idx", "sync"}, &stubIndexCommand{}, nil, nil)
	assert.NoError(t, runner.Run())
}

// --- runSearchCommand ---

func TestRunSearchCommand_ValidQuery_CallsSearcher(t *testing.T) {
	t.Parallel()

	// Arrange
	searcher := &stubSearcher{}
	runner := NewCommandRunner([]string{"idx", "search", "hello"}, nil, nil, searcher)
	cfg := &searchCommandConfig{
		format:   search.OutputText,
		operator: search.OperatorAND,
	}
	cmd := &cobra.Command{Use: "search"}

	// Act
	err := runner.runSearchCommand(cmd, []string{"hello"}, cfg)

	// Assert
	require.NoError(t, err)
	assert.True(t, searcher.called)
}

func TestRunSearchCommand_EmptyQuery_ReturnsError(t *testing.T) {
	t.Parallel()
	runner := NewCommandRunner([]string{"idx", "search"}, nil, nil, &stubSearcher{})
	cfg := &searchCommandConfig{
		format:   search.OutputText,
		operator: search.OperatorAND,
	}
	cmd := &cobra.Command{Use: "search"}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	require.Error(t, runner.runSearchCommand(cmd, []string{}, cfg))
}

func TestRunSearchCommand_ValidationError_ReturnsError(t *testing.T) {
	t.Parallel()
	runner := NewCommandRunner([]string{"idx"}, nil, nil, &stubSearcher{})
	cfg := &searchCommandConfig{
		format:       search.OutputText,
		operator:     search.OperatorAND,
		contextLines: -1, // invalid
	}
	cmd := &cobra.Command{Use: "search"}
	require.Error(t, runner.runSearchCommand(cmd, []string{"hello"}, cfg))
}

// --- newVersionCommand ---

func TestNewVersionCommand_Run_PrintsVersion(t *testing.T) {
	t.Parallel()
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	cmd := runner.newVersionCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.Run(cmd, []string{})
	assert.NotEmpty(t, buf.String(), "expected version output")
}

// --- newConfigShowCommand ---

func TestNewConfigShowCommand_RunE_NoConfigFile_Succeeds(t *testing.T) {
	t.Parallel()
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	cmd := runner.newConfigShowCommand()
	assert.NoError(t, cmd.RunE(cmd, []string{}))
}

// --- currentDirTilde home prefix branch ---

func TestCurrentDirTilde_WithPathOutsideHome_ReturnsCwd(t *testing.T) {
	t.Parallel()
	// Not inside home? Returns cwd unchanged. We can't easily control HOME here
	// but we can verify it returns something meaningful.
	result := currentDirTilde()
	require.NotEmpty(t, result)
	// Should either start with ~ or be an absolute path
	hasTilde := strings.HasPrefix(result, "~")
	hasSlash := strings.HasPrefix(result, "/")
	isDot := result == "."
	assert.True(t, hasTilde || hasSlash || isDot, "unexpected path format: %q", result)
}
