package cli

import "fmt"

type initCommand interface {
	Run() error
}

type CommandRunner struct {
	arguments   []string
	initCommand initCommand
}

// NewCommandRunner wires CLI arguments to command execution.
// Example: runner := NewCommandRunner(os.Args, initCommand)
func NewCommandRunner(arguments []string, initCommand initCommand) CommandRunner {
	return CommandRunner{
		arguments:   arguments,
		initCommand: initCommand,
	}
}

func (runner CommandRunner) Run() error {
	if len(runner.arguments) < 2 {
		return fmt.Errorf("missing command: got %v, expected one of [init]", runner.arguments)
	}

	if runner.arguments[1] != "init" {
		return fmt.Errorf("unsupported command %q: expected one of [init]", runner.arguments[1])
	}

	return runner.initCommand.Run()
}