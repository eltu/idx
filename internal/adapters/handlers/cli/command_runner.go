package cli

import (
	"fmt"
	"time"

	"go.uber.org/zap"

	"idx/internal/core/ports"
)

type runnableCommand interface {
	Run() error
}

type indexableCommand interface {
	Run() error
	Sync() error
	Status() error
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

// daemonableCommand defines methods for daemon control.
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
	logger := zap.L()
	logger.Info("starting command execution", zap.Strings("arguments", runner.arguments))

	if len(runner.arguments) < 2 {
		err := fmt.Errorf("missing command: got %v, expected one of [sync init status inspect watch destroy search]", runner.arguments)
		logger.Warn("invalid command invocation", zap.Error(err))
		return err
	}

	if !canExecuteWithCobra(runner.arguments[1]) {
		err := fmt.Errorf("unsupported command %q: expected one of [sync init status inspect watch destroy search]", runner.arguments[1])
		logger.Warn("unsupported command", zap.String("command", runner.arguments[1]), zap.Error(err))
		return err
	}

	root := runner.newRootCommand()
	root.SetArgs(runner.arguments[1:])

	err := root.Execute()
	if err != nil {
		logger.Error("command execution failed", zap.String("command", runner.arguments[1]), zap.Error(err))
		return err
	}

	logger.Info("command execution completed", zap.String("command", runner.arguments[1]))
	return nil
}

func canExecuteWithCobra(command string) bool {
	switch command {
	case "sync", "init", "status", "inspect", "watch", "destroy", "search", "daemon", "help", "--help", "-h":
		return true
	default:
		return false
	}
}
