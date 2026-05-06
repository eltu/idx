package cli_test

import (
	"testing"

	"idx/internal/adapters/handlers/cli"
)

func TestCommandRunnerRunRejectsUnsupportedCommand(t *testing.T) {
	initCommand := &fakeInitCommand{}
	destroyCommand := &fakeDestroyCommand{}
	searchCommand := &fakeSearchCommand{}
	daemonCommand := &fakeDaemonCommand{}
	runner := cli.NewCommandRunner([]string{"idx", "other"}, initCommand, destroyCommand, searchCommand, daemonCommand)

	if err := runner.Run(); err == nil {
		t.Fatal("expected an error, got nil")
	}
}
