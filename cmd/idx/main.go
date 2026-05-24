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
	featdaemon "idx/internal/features/daemon"
	featindexing "idx/internal/features/indexing"
	idxstorage "idx/internal/features/indexing/storage"
	idxtui "idx/internal/app/tui"
	featlifecycle "idx/internal/features/lifecycle"
	featread "idx/internal/features/read"
	featsearch "idx/internal/features/search"
	featskills "idx/internal/features/skills"
	sharedconfig "idx/internal/shared/config"
	sharedfs "idx/internal/shared/filesystem"
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

	logLevel := zapcore.ErrorLevel

	// .idx.yml log.level is the base; IDX_LOG_LEVEL env var takes precedence.
	if configLogLevel != "" {
		var parsedLevel zapcore.Level
		if err := parsedLevel.Set(strings.ToLower(configLogLevel)); err == nil {
			logLevel = parsedLevel
		}
	}

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
	writer := appcli.NewLineWriter(output)
	projectTree := sharedfs.NewOSProjectTree()
	matcherFactory := sharedfs.IgnoreMatcherFactory(sharedfs.NewGitIgnoreMatcherFactory())
	fileReader := sharedfs.NewOSFileReader()
	indexer := featindexing.NewBM25IndexService()
	indexRepo := idxstorage.NewBinaryIndexRepository()
	checksumRepo := idxstorage.NewDirectoryChecksumRepository()
	daemonStateRepo := featdaemon.NewDaemonStateRepository()
	processSpawner := featdaemon.NewOSProcessSpawner()
	inspectRunner := idxtui.NewInspectRunner()
	progressRunner := idxtui.NewInitProgressRunner()

	// Load project config from .idx.yml (git root) to wire service defaults.
	configRepo := sharedconfig.NewYAMLRepository()
	cwd, _ := os.Getwd()
	projectRoot := gitRootFrom(cwd)
	cfg, overrides, _ := configRepo.Load(projectRoot)
	configFilePath := configRepo.FilePath(projectRoot)

	matcherFactory = sharedfs.NewCompositeIgnoreMatcherFactory(
		matcherFactory,
		sharedfs.NewGlobIgnoreMatcherFactory(cfg.Index.Ignore),
	)

	// Break DI cycle: DaemonService → InitCommand → DaemonService (ProjectMonitorChecker).
	// Construct daemon first with nil initCommand, wire initCommand after it's built.
	daemonServiceImpl := featdaemon.NewDaemonService(daemonStateRepo, projectTree, writer, nil, processSpawner)
	initCommand := featindexing.NewInitCommandServiceWithProgress(projectTree, matcherFactory, writer, fileReader, indexer, indexRepo, checksumRepo, daemonServiceImpl, inspectRunner, progressRunner)
	initAdapter := appcli.NewInitCommandAdapter(initCommand, projectTree)
	daemonServiceImpl.SetInitCommand(initAdapter)

	daemonService := appcli.NewDaemonServiceAdapter(daemonServiceImpl)
	destroyCommand := featlifecycle.NewDestroyCommandService(projectTree, writer)
	readLogRepo := featread.NewReadLogRepository()
	searchCommand := featsearch.NewSearchCommandService(projectTree, writer, fileReader, indexRepo).
		WithTuning(featsearch.SearchServiceOptions{
			BM25K1:           cfg.BM25.K1,
			BM25B:            cfg.BM25.B,
			ProximityWeight:  cfg.BM25.ProximityWeight,
			PopularityWeight: cfg.BM25.PopularityWeight,
			MaxWorkers:       cfg.Search.MaxWorkers,
			CacheTTL:         cfg.Search.CacheTTL,
		}).
		WithReadLog(readLogRepo)
	skillsInstaller := featskills.NewOSSkillsInstaller()
	skillsService := featskills.NewSkillsInstallService(skillsInstaller, output)
	fileStreamer := featread.NewOSFileStreamer()
	readService := featread.NewReadCommandService(projectTree, fileStreamer, writer).
		WithReadLog(readLogRepo)
	runner := appcli.NewCommandRunner(arguments, initCommand, destroyCommand, searchCommand, daemonService).
		WithBuildInfo(appcli.BuildInfo{Version: version, BuildDate: buildDate}).
		WithQuietToggle(multiQuiet{writer, progressRunner}).
		WithSkillsCommand(skillsService).
		WithReadCommand(readService).
		WithConfig(cfg, configFilePath, overrides)

	return runner.Run()
}

// earlyLoadConfigLogLevel reads only the log.level field from .idx.yml so the
// logger can be initialised before the full DI graph is wired in run().
func earlyLoadConfigLogLevel(projectRoot string) string {
	configRepo := sharedconfig.NewYAMLRepository()
	cfg, _, err := configRepo.Load(projectRoot)
	if err != nil {
		return ""
	}
	return cfg.Log.Level
}
