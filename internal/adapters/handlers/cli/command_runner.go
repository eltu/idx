package cli

import (
	"fmt"
	"strings"
)

type runnableCommand interface {
	Run() error
}

type searchableCommand interface {
	Run(query string) error
}

type CommandRunner struct {
	arguments      []string
	initCommand    runnableCommand
	destroyCommand runnableCommand
	searchCommand  searchableCommand
}

// NewCommandRunner wires CLI arguments to command execution.
// Example: runner := NewCommandRunner(os.Args, initCommand, destroyCommand, searchCommand)
func NewCommandRunner(arguments []string, initCommand runnableCommand, destroyCommand runnableCommand, searchCommand searchableCommand) CommandRunner {
	return CommandRunner{
		arguments:      arguments,
		initCommand:    initCommand,
		destroyCommand: destroyCommand,
		searchCommand:  searchCommand,
	}
}

func (runner CommandRunner) Run() error {
	if len(runner.arguments) < 2 {
		return fmt.Errorf("missing command: got %v, expected one of [init destroy search]", runner.arguments)
	}

	switch runner.arguments[1] {
	case "init":
		return runner.initCommand.Run()
	case "destroy":
		return runner.destroyCommand.Run()
	case "search":
		return runner.runSearch()
	default:
		return fmt.Errorf("unsupported command %q: expected one of [init destroy search]", runner.arguments[1])
	}
}

func (runner CommandRunner) runSearch() error {
	if len(runner.arguments) < 3 {
		return fmt.Errorf("missing search query: got %v, expected idx search <terms>", runner.arguments)
	}

	return runner.searchCommand.Run(strings.Join(runner.arguments[2:], " "))
}
