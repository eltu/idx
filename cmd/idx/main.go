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

	appcli "idx/internal/app/cli"
	"idx/internal/app/cli/remote"
	idxtui "idx/internal/app/tui"
	featdaemon "idx/internal/features/daemon"
	sharedconfig "idx/internal/shared/config"
	idxipc "idx/internal/shared/ipc"
)

const unknownProject = "unknown-project"

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
	// Load config early so log.level in .idx.yml affects the logger.
	cwd, _ := os.Getwd()
	projectRoot := gitRootFrom(cwd)
	earlyConfig := earlyLoadConfigLogLevel(projectRoot)

	logger, err := newLogger(earlyConfig)
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

func newLogger(configLogLevel string) (*zap.Logger, error) {
	logPath, err := loggerOutputPath()
	if err != nil {
		return nil, err
	}
	logLevel, err := resolveLogLevel(configLogLevel)
	if err != nil {
		return nil, err
	}
	return buildZapLogger(logPath, logLevel)
}

// resolveLogLevel applies configLogLevel as base, with IDX_LOG_LEVEL env var taking precedence.
// Defaults to zapcore.ErrorLevel when neither source provides a valid level.
func resolveLogLevel(configLogLevel string) (zapcore.Level, error) {
	logLevel := zapcore.ErrorLevel
	if configLogLevel != "" {
		var parsed zapcore.Level
		if err := parsed.Set(strings.ToLower(configLogLevel)); err == nil {
			logLevel = parsed
		}
	}
	if envLevel := strings.TrimSpace(os.Getenv("IDX_LOG_LEVEL")); envLevel != "" {
		var parsed zapcore.Level
		if err := parsed.Set(strings.ToLower(envLevel)); err != nil {
			return logLevel, fmt.Errorf("invalid IDX_LOG_LEVEL %q: expected one of [debug info warn error dpanic panic fatal]", envLevel)
		}
		logLevel = parsed
	}
	return logLevel, nil
}

func buildZapLogger(logPath string, logLevel zapcore.Level) (*zap.Logger, error) {
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
		return unknownProject
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
		return unknownProject
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

func firstNonFlagArg(arguments []string) string {
	for _, arg := range arguments[1:] {
		if arg != "" && arg[0] != '-' {
			return arg
		}
	}
	return ""
}

func isServerCommand(arguments []string) bool {
	nonFlagArgs := make([]string, 0, len(arguments))
	for _, arg := range arguments[1:] {
		if arg != "" && arg[0] != '-' {
			nonFlagArgs = append(nonFlagArgs, arg)
		}
	}
	if len(nonFlagArgs) < 2 || nonFlagArgs[1] != "run" {
		return false
	}
	return nonFlagArgs[0] == "agent" || nonFlagArgs[0] == "server"
}

// isInitCommand returns true for "idx init [flags]".
// idx init is a bootstrap operation that must run before the server exists,
// so it executes in-process rather than delegating to the server via RPC.
// See ADR-0022 for rationale.
func isInitCommand(arguments []string) bool {
	return firstNonFlagArg(arguments) == "init"
}

func run(arguments []string, output io.Writer) error {
	if isServerCommand(arguments) || isInitCommand(arguments) {
		return runServer(arguments, output)
	}
	return runClient(arguments, output)
}

func runServer(arguments []string, output io.Writer) error {
	d, err := sharedDeps(output)
	if err != nil {
		return err
	}
	w := serverWiring{
		shared:   d,
		indexing: buildIndexingDeps(d),
		read:     buildReadDeps(d),
		tuning:   buildSearchTuning(d.cfg),
	}
	searchCmd := buildSearchDeps(w.shared, w.read, w.tuning)
	relatedSvc := buildRelatedDeps(w.shared, w.read)
	skillsSvc := buildSkillsDeps(w.shared)
	indexServer := buildIndexServer(w)
	return appcli.NewCommandRunner(arguments, w.indexing.initCommand, w.indexing.destroyCommand, searchCmd).
		WithBuildInfo(appcli.BuildInfo{Version: version, BuildDate: buildDate}).
		WithQuietToggle(multiQuiet{d.writer, w.indexing.progressRunner}).
		WithSkillsCommand(skillsSvc).
		WithReadCommand(w.read.readService).
		WithRelatedCommand(relatedSvc).
		WithIndexServer(indexServer).
		WithConfig(d.cfg, d.configFilePath, d.overrides).
		Run()
}

func runClient(arguments []string, output io.Writer) error {
	d, err := sharedDeps(output)
	if err != nil {
		return err
	}
	progressRunner := idxtui.NewInitProgressRunner()
	serverDaemon := featdaemon.NewServerDaemonService(
		featdaemon.NewServerStateRepository(),
		featdaemon.NewOSServerSpawner(),
		d.writer,
	)
	skillsSvc := buildSkillsDeps(d)
	socketPath := idxipc.SocketPath(d.projectRoot)
	client := remote.NewSocketClient(socketPath)
	inspectRunner := idxtui.NewInspectRunner()
	return appcli.NewCommandRunner(arguments,
		remote.NewRemoteIndexCommand(client, d.writer, inspectRunner),
		remote.NewRemoteDestroyCommand(client, d.writer),
		remote.NewRemoteSearcher(client, d.writer),
	).
		WithBuildInfo(appcli.BuildInfo{Version: version, BuildDate: buildDate}).
		WithQuietToggle(multiQuiet{d.writer, progressRunner}).
		WithSkillsCommand(skillsSvc).
		WithReadCommand(remote.NewRemoteReader(client, d.writer)).
		WithRelatedCommand(remote.NewRemoteRelatedCommand(client, d.writer)).
		WithServerManager(appcli.NewServerDaemonAdapter(serverDaemon)).
		WithConfig(d.cfg, d.configFilePath, d.overrides).
		WithProjectRoot(d.projectRoot).
		WithConfigCommand(remote.NewRemoteConfigCommand(client, d.writer)).
		Run()
}

// earlyLoadConfigLogLevel reads only the log.level field from .idx.yml so the
// logger can be initialized before the full DI graph is wired in run().
func earlyLoadConfigLogLevel(projectRoot string) string {
	configRepo := sharedconfig.NewYAMLRepository()
	cfg, _, err := configRepo.Load(projectRoot)
	if err != nil {
		return ""
	}
	return cfg.Log.Level
}
