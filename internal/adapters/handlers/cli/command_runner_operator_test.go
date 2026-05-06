package cli_test

import (
	"testing"

	"idx/internal/adapters/handlers/cli"
	"idx/internal/core/ports"
)

func TestCommandRunnerRunAllowsHelpCommand(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	daemonCommand := &fakeDaemonCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "help"}, initCommand, destroyCommand, searchCommand, daemonCommand)

	if err := runner.Run(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if initCommand.runCalls != 0 || initCommand.syncCalls != 0 || initCommand.inspectCalls != 0 || destroyCommand.runCalls != 0 || searchCommand.runCalls != 0 {
		t.Fatalf("expected no business command calls on help, got init=%+v destroy=%d search=%d", initCommand, destroyCommand.runCalls, searchCommand.runCalls)
	}
}

func TestCommandRunnerRunAllowsHelpFlag(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	daemonCommand := &fakeDaemonCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "--help"}, initCommand, destroyCommand, searchCommand, daemonCommand)

	if err := runner.Run(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestCommandRunnerRunPassesDefaultANDOperator(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	daemonCommand := &fakeDaemonCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "search", "needle"}, initCommand, destroyCommand, searchCommand, daemonCommand)

	if err := runner.Run(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if searchCommand.lastOptions.Operator != ports.SearchOperatorAND {
		t.Fatalf("expected default operator AND, got %q", searchCommand.lastOptions.Operator)
	}
}

func TestCommandRunnerRunPassesOROperator(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	daemonCommand := &fakeDaemonCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "search", "--operator", "OR", "needle"}, initCommand, destroyCommand, searchCommand, daemonCommand)

	if err := runner.Run(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if searchCommand.lastOptions.Operator != ports.SearchOperatorOR {
		t.Fatalf("expected operator OR, got %q", searchCommand.lastOptions.Operator)
	}
}

func TestCommandRunnerRunRejectsUnsupportedOperator(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	daemonCommand := &fakeDaemonCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "search", "--operator", "NOT", "needle"}, initCommand, destroyCommand, searchCommand, daemonCommand)

	if err := runner.Run(); err == nil {
		t.Fatal("expected error for unsupported --operator value, got nil")
	}
}
