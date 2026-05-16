package cli

import (
	"time"

	"go.uber.org/zap"

	"idx/internal/core/domain"
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

type readableCommand interface {
	RunWithOptions(filePath string, fromLine, toLine int) error
}

// BuildInfo holds version and build date injected at compile time via -ldflags.
type BuildInfo struct {
	Version   string
	BuildDate string
}

type CommandRunner struct {
	arguments       []string
	indexCommand    indexableCommand
	destroyCommand  runnableCommand
	searchCommand   searchableCommand
	readCommand     readableCommand
	daemonService   daemonableCommand
	skillsCommand   skillsableCommand
	buildInfo       BuildInfo
	quietToggle     interface{ SetQuiet(bool) }
	config          domain.IdxConfig
	configFilePath  string
	configOverrides []string
}

// daemonableCommand defines methods for daemon control.
type daemonableCommand interface {
	Enable(projectPath string) error
	Disable(projectPath string) error
	Status() error
}

// NewCommandRunner wires CLI arguments to command execution.
// Initialises config with DefaultIdxConfig so flag defaults are valid even
// when WithConfig is not called (e.g. in unit tests).
// Example: runner := NewCommandRunner(os.Args, initCommand, destroyCommand, searchCommand, daemonService).
func NewCommandRunner(arguments []string, indexCommand indexableCommand, destroyCommand runnableCommand, searchCommand searchableCommand, daemonService daemonableCommand) CommandRunner {
	return CommandRunner{
		arguments:      arguments,
		indexCommand:   indexCommand,
		destroyCommand: destroyCommand,
		searchCommand:  searchCommand,
		daemonService:  daemonService,
		config:         domain.DefaultIdxConfig(),
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

// WithReadCommand wires the file reader so 'idx read <path>' works.
// Example: runner = runner.WithReadCommand(readService).
func (runner CommandRunner) WithReadCommand(r readableCommand) CommandRunner {
	runner.readCommand = r
	return runner
}

// WithQuietToggle wires a quietable writer so the --quiet persistent flag can
// suppress informational output at runtime without changing the output stream.
// Example: runner = runner.WithQuietToggle(writer).
func (runner CommandRunner) WithQuietToggle(t interface{ SetQuiet(bool) }) CommandRunner {
	runner.quietToggle = t
	return runner
}

// WithConfig attaches the resolved project config so commands can use
// file-level defaults and 'idx config show' can display override details.
// Example: runner = runner.WithConfig(cfg, "/project/.idx.yml", []string{"search.format"}).
func (runner CommandRunner) WithConfig(cfg domain.IdxConfig, filePath string, overrides []string) CommandRunner {
	runner.config = cfg
	runner.configFilePath = filePath
	runner.configOverrides = overrides
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
	case "sync", "init", "status", "inspect", "read", "watch", "destroy", "search", "daemon", "version", "skills", "config", "help", "--help", "-h", "--version", "-v":
		return true
	default:
		return false
	}
}
