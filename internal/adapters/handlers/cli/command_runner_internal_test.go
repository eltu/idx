package cli

import (
	"testing"
	"time"

	"idx/internal/core/ports"
)

type noOpIndexCommand struct{}

func (noOpIndexCommand) Run() error           { return nil }
func (noOpIndexCommand) Sync() error          { return nil }
func (noOpIndexCommand) Inspect(string) error { return nil }
func (noOpIndexCommand) Watch(bool, time.Duration) error {
	return nil
}

type noOpDestroyCommand struct{}

func (noOpDestroyCommand) Run() error { return nil }

type noOpSearchCommand struct{}

func (noOpSearchCommand) RunWithOptions(string, ports.SearchOptions) error { return nil }

type noOpDaemonCommand struct{}

func (noOpDaemonCommand) Enable(string) error  { return nil }
func (noOpDaemonCommand) Disable(string) error { return nil }
func (noOpDaemonCommand) Status() error        { return nil }

func TestCommandRunnerRunRejectsMissingCommand(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, noOpIndexCommand{}, noOpDestroyCommand{}, noOpSearchCommand{}, noOpDaemonCommand{})
	if err := runner.Run(); err == nil {
		t.Fatal("expected missing command error")
	}
}

func TestParseInspectArgumentsInternal(t *testing.T) {
	if _, err := parseInspectArguments([]string{}); err == nil {
		t.Fatal("expected error for missing inspect path")
	}

	if _, err := parseInspectArguments([]string{"--flag"}); err == nil {
		t.Fatal("expected error for flag-like inspect path")
	}

	path, err := parseInspectArguments([]string{"internal"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if path != "internal" {
		t.Fatalf("expected internal, got %q", path)
	}
}
