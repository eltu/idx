package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"idx/internal/adapters/handlers/cli"
	"idx/internal/adapters/handlers/tui"
	"idx/internal/adapters/repository"
	"idx/internal/core/services/daemon"
	"idx/internal/core/services/indexing"
	"idx/internal/core/services/lifecycle"
	"idx/internal/core/services/search"
)

var exitProcess = os.Exit

type multiQuiet struct {
	a, b interface{ SetQuiet(bool) }
}

func (m multiQuiet) SetQuiet(q bool) {
	m.a.SetQuiet(q)
	m.b.SetQuiet(q)
}

// version and buildDate are injected at build time via -ldflags.
var (
	version   = "dev"
	buildDate = "unknown"
)

func main() {
	logger, err := newLogger()
	if err != nil {
		logger = zap.NewNop()
		zap.ReplaceGlobals(logger)
	}
	defer func() { _ = logger.Sync() }()

	if err := run(os.Args, os.Stdout); err != nil {
		logger.Error("command failed", zap.Error(err), zap.Strings("arguments", os.Args))
		if msg := err.Error(); msg != "" {
			fmt.Fprintln(os.Stderr, msg)
		}
		exitProcess(1)
	}
}

func newLogger() (*zap.Logger, error) {
	logPath, err := loggerOutputPath()
	if err != nil {
		return nil, err
	}

	logLevel := zapcore.ErrorLevel

	if envLevel := strings.TrimSpace(os.Getenv("IDX_LOG_LEVEL")); envLevel != "" {
		var parsedLevel zapcore.Level
		if err := parsedLevel.Set(strings.ToLower(envLevel)); err != nil {
			return nil, fmt.Errorf("invalid IDX_LOG_LEVEL %q: expected one of [debug info warn error dpanic panic fatal]", envLevel)
		}
		logLevel = parsedLevel
	}

	encoderConfig := zap.NewProductionEncoderConfig()
	rotatingWriter := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    1,
		MaxBackups: 5,
	}

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.AddSync(rotatingWriter),
		logLevel,
	)

	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	zap.ReplaceGlobals(logger)
	return logger, nil
}

func loggerOutputPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve user home directory: %w", err)
	}

	currentDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to resolve current directory for logger path: %w", err)
	}

	projectName := projectNameFromDir(currentDir)
	logsDir := filepath.Join(homeDir, ".idx", "logs", projectName)
	if err := os.MkdirAll(logsDir, 0o750); err != nil {
		return "", fmt.Errorf("failed to create logs directory %q: %w", logsDir, err)
	}

	return filepath.Join(logsDir, "idx.log"), nil
}

func projectNameFromDir(currentDir string) string {
	rootDir := gitRootFrom(currentDir)
	baseName := filepath.Base(rootDir)
	return sanitizePathSegment(baseName)
}

func gitRootFrom(startDir string) string {
	currentDir := startDir
	for {
		if _, err := os.Stat(filepath.Join(currentDir, ".git")); err == nil {
			return currentDir
		}

		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			return startDir
		}

		currentDir = parentDir
	}
}

func sanitizePathSegment(name string) string {
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "unknown-project"
	}

	b := strings.Builder{}
	b.Grow(len(name))
	for i := 0; i < len(name); i++ {
		char := name[i]
		if isPathSafeChar(char) {
			b.WriteByte(char)
			continue
		}

		b.WriteByte('_')
	}

	clean := strings.Trim(b.String(), "._-")
	if clean == "" {
		return "unknown-project"
	}

	return clean
}

func isPathSafeChar(char byte) bool {
	if char >= 'a' && char <= 'z' {
		return true
	}

	if char >= 'A' && char <= 'Z' {
		return true
	}

	if char >= '0' && char <= '9' {
		return true
	}

	return char == '-' || char == '_' || char == '.'
}

func run(arguments []string, output io.Writer) error {
	writer := cli.NewLineWriter(output)
	projectTree := repository.NewOSProjectTree()
	matcherFactory := repository.NewGitIgnoreMatcherFactory()
	fileReader := repository.NewOSFileReader()
	indexer := indexing.NewBM25IndexService()
	indexRepo := repository.NewBinaryIndexRepository()
	checksumRepo := repository.NewDirectoryChecksumRepository()
	daemonStateRepo := repository.NewDaemonStateRepository()
	processSpawner := &repository.OSProcessSpawner{}
	inspectRunner := tui.NewInspectRunner()
	progressRunner := tui.NewInitProgressRunner()

	initCommand := indexing.NewInitCommandServiceWithProgress(projectTree, matcherFactory, writer, fileReader, indexer, indexRepo, checksumRepo, daemonStateRepo, inspectRunner, progressRunner)

	initAdapter := cli.NewInitCommandAdapter(initCommand, projectTree)
	daemonServiceImpl := daemon.NewDaemonService(daemonStateRepo, projectTree, writer, initAdapter, processSpawner)
	daemonService := cli.NewDaemonServiceAdapter(daemonServiceImpl)

	destroyCommand := lifecycle.NewDestroyCommandService(projectTree, writer)
	searchCommand := search.NewSearchCommandService(projectTree, writer, fileReader, indexRepo)
	runner := cli.NewCommandRunner(arguments, initCommand, destroyCommand, searchCommand, daemonService).
		WithBuildInfo(cli.BuildInfo{Version: version, BuildDate: buildDate}).
		WithQuietToggle(multiQuiet{writer, progressRunner})

	return runner.Run()
}

