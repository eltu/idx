package cli

import (
	"fmt"
	"time"

	"idx/internal/core/ports"
)

type runnableCommand interface {
	Run() error
}

type indexableCommand interface {
	Run() error
	Sync() error
	Inspect(indexPath string) error
	Watch(showUpdatedFiles bool, debounce time.Duration) error
}

type searchableCommand interface {
	RunWithOptions(query string, options ports.SearchOptions) error
}

type CommandRunner struct {
	arguments      []string
	indexCommand   indexableCommand
	destroyCommand runnableCommand
	searchCommand  searchableCommand
	daemonService  daemonableCommand
}

// daemonableCommand define métodos para controle do daemon.
type daemonableCommand interface {
	Enable(projectPath string) error
	Disable(projectPath string) error
	Status() error
}

// NewCommandRunner wires CLI arguments to command execution.
// Example: runner := NewCommandRunner(os.Args, initCommand, destroyCommand, searchCommand, daemonService).
func NewCommandRunner(arguments []string, indexCommand indexableCommand, destroyCommand runnableCommand, searchCommand searchableCommand, daemonService daemonableCommand) CommandRunner {
	return CommandRunner{
		arguments:      arguments,
		indexCommand:   indexCommand,
		destroyCommand: destroyCommand,
		searchCommand:  searchCommand,
		daemonService:  daemonService,
	}
}

// Run dispatches the CLI command based on the first argument.
// Example: err := runner.Run().
func (runner CommandRunner) Run() error {
	if len(runner.arguments) < 2 {
		return fmt.Errorf("missing command: got %v, expected one of [sync init inspect watch destroy search]", runner.arguments)
	}

	if !canExecuteWithCobra(runner.arguments[1]) {
		return fmt.Errorf("unsupported command %q: expected one of [sync init inspect watch destroy search]", runner.arguments[1])
	}

	root := runner.newRootCommand()
	root.SetArgs(runner.arguments[1:])
	return root.Execute()
}

func canExecuteWithCobra(command string) bool {
	switch command {
	case "sync", "init", "inspect", "watch", "destroy", "search", "daemon", "help", "--help", "-h":
		return true
	default:
		return false
	}
}
