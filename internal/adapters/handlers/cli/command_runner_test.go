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

type fakeSearchCommand struct {
	runCalls  int
	lastQuery string
}

func (command *fakeSearchCommand) Run(query string) error {
	command.runCalls++
	command.lastQuery = query
	return nil
}

func TestCommandRunnerRunExecutesInitCommand(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "init"}, initCommand, destroyCommand, searchCommand)

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

	if searchCommand.runCalls != 0 {
		t.Fatalf("expected 0 search calls, got %d", searchCommand.runCalls)
	}
}

func TestCommandRunnerRunExecutesDestroyCommand(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "destroy"}, initCommand, destroyCommand, searchCommand)

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

	if searchCommand.runCalls != 0 {
		t.Fatalf("expected 0 search calls, got %d", searchCommand.runCalls)
	}
}

func TestCommandRunnerRunExecutesSearchCommand(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "search", "needle", "term"}, initCommand, destroyCommand, searchCommand)

	err := runner.Run()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if searchCommand.runCalls != 1 {
		t.Fatalf("expected 1 search call, got %d", searchCommand.runCalls)
	}

	if searchCommand.lastQuery != "needle term" {
		t.Fatalf("expected query %q, got %q", "needle term", searchCommand.lastQuery)
	}
}

func TestCommandRunnerRunRejectsSearchWithoutQuery(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "search"}, initCommand, destroyCommand, searchCommand)

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

	if searchCommand.runCalls != 0 {
		t.Fatalf("expected 0 search calls, got %d", searchCommand.runCalls)
	}
}

func TestCommandRunnerRunRejectsUnsupportedCommand(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "other"}, initCommand, destroyCommand, searchCommand)

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

	if searchCommand.runCalls != 0 {
		t.Fatalf("expected 0 search calls, got %d", searchCommand.runCalls)
	}
}
