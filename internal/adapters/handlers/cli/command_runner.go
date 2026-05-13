package cli

import (
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

// BuildInfo holds version and build date injected at compile time via -ldflags.
type BuildInfo struct {
	Version   string
	BuildDate string
}

type CommandRunner struct {
	arguments      []string
	indexCommand   indexableCommand
	destroyCommand runnableCommand
	searchCommand  searchableCommand
	daemonService  daemonableCommand
	skillsCommand  skillsableCommand
	buildInfo      BuildInfo
	quietToggle    interface{ SetQuiet(bool) }
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

// WithBuildInfo attaches version and build date to the runner for the version command.
// Example: runner = runner.WithBuildInfo(cli.BuildInfo{Version: "v1.0.0", BuildDate: "2026-05-05"}).
func (runner CommandRunner) WithBuildInfo(info BuildInfo) CommandRunner {
	runner.buildInfo = info
	return runner
}

// WithSkillsCommand wires the skills installer so 'idx skills install' works.
// Example: runner = runner.WithSkillsCommand(skillsService).
func (runner CommandRunner) WithSkillsCommand(s skillsableCommand) CommandRunner {
	runner.skillsCommand = s
	return runner
}

// WithQuietToggle wires a quietable writer so the --quiet persistent flag can
// suppress informational output at runtime without changing the output stream.
// Example: runner = runner.WithQuietToggle(writer).
func (runner CommandRunner) WithQuietToggle(t interface{ SetQuiet(bool) }) CommandRunner {
	runner.quietToggle = t
	return runner
}

// Run dispatches the CLI command based on the first argument.
// Example: err := runner.Run().
func (runner CommandRunner) Run() error {
	logger := zap.L()
	logger.Info("starting command execution", zap.Strings("arguments", runner.arguments))

	args := runner.arguments[1:]
	root := runner.newRootCommand()
	root.SetArgs(args)

	command := ""
	if len(args) > 0 {
		command = args[0]
	}

	err := root.Execute()
	if err != nil {
		logger.Error("command execution failed", zap.String("command", command), zap.Error(err))
		return err
	}

	logger.Info("command execution completed", zap.String("command", command))
	return nil
}

func canExecuteWithCobra(command string) bool {
	switch command {
	case "sync", "init", "status", "inspect", "watch", "destroy", "search", "daemon", "version", "skills", "help", "--help", "-h", "--version", "-v":
		return true
	default:
		return false
	}
}
