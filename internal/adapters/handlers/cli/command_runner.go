package cli

import "fmt"

type runnableCommand interface {
	Run() error
}

type CommandRunner struct {
	arguments      []string
	initCommand    runnableCommand
	destroyCommand runnableCommand
}

// NewCommandRunner wires CLI arguments to command execution.
// Example: runner := NewCommandRunner(os.Args, initCommand, destroyCommand)
func NewCommandRunner(arguments []string, initCommand runnableCommand, destroyCommand runnableCommand) CommandRunner {
	return CommandRunner{
		arguments:      arguments,
		initCommand:    initCommand,
		destroyCommand: destroyCommand,
	}
}

func (runner CommandRunner) Run() error {
	if len(runner.arguments) < 2 {
		return fmt.Errorf("missing command: got %v, expected one of [init destroy]", runner.arguments)
	}

	switch runner.arguments[1] {
	case "init":
		return runner.initCommand.Run()
	case "destroy":
		return runner.destroyCommand.Run()
	default:
		return fmt.Errorf("unsupported command %q: expected one of [init destroy]", runner.arguments[1])
	}
}
