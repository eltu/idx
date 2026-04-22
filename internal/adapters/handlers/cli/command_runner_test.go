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

type fakeDestroyCommand struct {
	runCalls int
}

func (command *fakeDestroyCommand) Run() error {
	command.runCalls++
	return nil
}

func TestCommandRunnerRunExecutesInitCommand(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "init"}, initCommand, destroyCommand)

	err := runner.Run()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if initCommand.runCalls != 1 {
		t.Fatalf("expected 1 run call, got %d", initCommand.runCalls)
	}

	if destroyCommand.runCalls != 0 {
		t.Fatalf("expected 0 destroy calls, got %d", destroyCommand.runCalls)
	}
}

func TestCommandRunnerRunExecutesDestroyCommand(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "destroy"}, initCommand, destroyCommand)

	err := runner.Run()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if destroyCommand.runCalls != 1 {
		t.Fatalf("expected 1 destroy call, got %d", destroyCommand.runCalls)
	}

	if initCommand.runCalls != 0 {
		t.Fatalf("expected 0 init calls, got %d", initCommand.runCalls)
	}
}

func TestCommandRunnerRunRejectsUnsupportedCommand(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "search"}, initCommand, destroyCommand)

	err := runner.Run()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if initCommand.runCalls != 0 {
		t.Fatalf("expected 0 init calls, got %d", initCommand.runCalls)
	}

	if destroyCommand.runCalls != 0 {
		t.Fatalf("expected 0 destroy calls, got %d", destroyCommand.runCalls)
	}
}
