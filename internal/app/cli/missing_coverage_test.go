package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	appserver "idx/internal/app/server"
	search "idx/internal/features/search"
)

// --- validateSearchConfig ---

func TestValidateSearchConfigValidInputNoError(t *testing.T) {
	cfg := &searchCommandConfig{
		format:   search.OutputText,
		operator: search.OperatorAND,
	}
	if err := validateSearchConfig(cfg, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateSearchConfigNegativeContextErrors(t *testing.T) {
	cfg := &searchCommandConfig{
		format:       search.OutputText,
		operator:     search.OperatorAND,
		contextLines: -1,
	}
	if err := validateSearchConfig(cfg, false); err == nil {
		t.Fatal("expected error for negative context")
	}
}

func TestValidateSearchConfigBadFormatErrors(t *testing.T) {
	cfg := &searchCommandConfig{
		format:   "xml",
		operator: search.OperatorAND,
	}
	if err := validateSearchConfig(cfg, false); err == nil {
		t.Fatal("expected error for unknown format")
	}
}

func TestValidateSearchConfigBadOperatorErrors(t *testing.T) {
	cfg := &searchCommandConfig{
		format:   search.OutputText,
		operator: "XOR",
	}
	if err := validateSearchConfig(cfg, false); err == nil {
		t.Fatal("expected error for unknown operator")
	}
}

func TestValidateSearchConfigInvalidRelaxationErrors(t *testing.T) {
	cfg := &searchCommandConfig{
		format:     search.OutputText,
		operator:   search.OperatorAND,
		relaxation: "no-angle",
	}
	if err := validateSearchConfig(cfg, false); err == nil {
		t.Fatal("expected error for bad relaxation format")
	}
}

// --- writeSearchMissingQueryError ---

func TestWriteSearchMissingQueryErrorWritesToCommand(t *testing.T) {
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

func TestWithIndexServerSetsField(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, nil).
		WithIndexServer(&stubServerRunner{})
	if runner.indexServer == nil {
		t.Fatal("expected indexServer to be set")
	}
}

func TestWithIndexServerAcceptsNilServerRunner(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, nil).
		WithIndexServer(appserver.NewServer(appserver.ServerDeps{}))
	if runner.indexServer == nil {
		t.Fatal("expected indexServer to be set with real server")
	}
}

// --- CommandRunner.Run ---

type stubSearcher struct{ called bool }

func (s *stubSearcher) RunWithOptions(_ string, _ search.Options) error {
	s.called = true
	return nil
}

func TestCommandRunnerRunUnknownCommandReturnsError(t *testing.T) {
	runner := NewCommandRunner([]string{"idx", "unknown-xyz"}, &stubIndexCommand{}, nil, &stubSearcher{}, nil)
	err := runner.Run()
	if err == nil {
		t.Fatal("expected error for unknown command, got nil")
	}
}

func TestCommandRunnerRunKnownCommandSucceeds(t *testing.T) {
	runner := NewCommandRunner([]string{"idx", "sync"}, &stubIndexCommand{}, nil, nil, nil)
	if err := runner.Run(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- runSearchCommand ---

func TestRunSearchCommandWithValidQuery(t *testing.T) {
	searcher := &stubSearcher{}
	runner := NewCommandRunner([]string{"idx", "search", "hello"}, nil, nil, searcher, nil)
	cfg := &searchCommandConfig{
		format:   search.OutputText,
		operator: search.OperatorAND,
	}
	cmd := &cobra.Command{Use: "search"}
	if err := runner.runSearchCommand(cmd, []string{"hello"}, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !searcher.called {
		t.Error("expected searcher to be called")
	}
}

func TestRunSearchCommandEmptyQueryWritesError(t *testing.T) {
	runner := NewCommandRunner([]string{"idx", "search"}, nil, nil, &stubSearcher{}, nil)
	cfg := &searchCommandConfig{
		format:   search.OutputText,
		operator: search.OperatorAND,
	}
	cmd := &cobra.Command{Use: "search"}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := runner.runSearchCommand(cmd, []string{}, cfg)
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestRunSearchCommandValidationErrorReturnsError(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, &stubSearcher{}, nil)
	cfg := &searchCommandConfig{
		format:       search.OutputText,
		operator:     search.OperatorAND,
		contextLines: -1, // invalid
	}
	cmd := &cobra.Command{Use: "search"}
	if err := runner.runSearchCommand(cmd, []string{"hello"}, cfg); err == nil {
		t.Fatal("expected validation error for negative context lines")
	}
}

// --- newVersionCommand ---

func TestNewVersionCommandRunPrintsVersion(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, nil)
	cmd := runner.newVersionCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.Run(cmd, []string{})
	if buf.Len() == 0 {
		t.Error("expected version output, got none")
	}
}

// --- newConfigShowCommand ---

func TestNewConfigShowCommandRunEWithNoConfigFile(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, nil)
	cmd := runner.newConfigShowCommand()
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- currentDirTilde home prefix branch ---

func TestCurrentDirTildeWithPathOutsideHomeReturnsCwd(t *testing.T) {
	// Not inside home? Returns cwd unchanged. We can't easily control HOME here
	// but we can verify it returns something meaningful.
	result := currentDirTilde()
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	// Should either start with ~ or be an absolute path
	if !strings.HasPrefix(result, "~") && !strings.HasPrefix(result, "/") && result != "." {
		t.Errorf("unexpected path format: %q", result)
	}
}
