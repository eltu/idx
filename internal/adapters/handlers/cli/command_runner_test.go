package cli_test

import (
	"testing"

	"idx/internal/adapters/handlers/cli"
)

type fakeInitCommand struct {
	runCalls int
}

func (command *fakeInitCommand) Run() error {
	command.runCalls++
	return nil
}

func TestCommandRunnerRunExecutesInitCommand(t *testing.T) {
	command := &fakeInitCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "init"}, command)

	err := runner.Run()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if command.runCalls != 1 {
		t.Fatalf("expected 1 run call, got %d", command.runCalls)
	}
}

func TestCommandRunnerRunRejectsUnsupportedCommand(t *testing.T) {
	command := &fakeInitCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "search"}, command)

	err := runner.Run()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if command.runCalls != 0 {
		t.Fatalf("expected 0 run calls, got %d", command.runCalls)
	}
}