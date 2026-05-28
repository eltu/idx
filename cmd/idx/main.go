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
	appserver "idx/internal/app/server"
	idxtui "idx/internal/app/tui"
	featdaemon "idx/internal/features/daemon"
	featindexing "idx/internal/features/indexing"
	idxstorage "idx/internal/features/indexing/storage"
	featlifecycle "idx/internal/features/lifecycle"
	featread "idx/internal/features/read"
	featsearch "idx/internal/features/search"
	featskills "idx/internal/features/skills"
	sharedconfig "idx/internal/shared/config"
	sharedfs "idx/internal/shared/filesystem"
	idxipc "idx/internal/shared/ipc"
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

func isServerCommand(arguments []string) bool {
	nonFlagArgs := make([]string, 0, len(arguments))
	for _, arg := range arguments[1:] {
		if arg != "" && arg[0] != '-' {
			nonFlagArgs = append(nonFlagArgs, arg)
		}
	}
	return len(nonFlagArgs) >= 2 && nonFlagArgs[0] == "server" && nonFlagArgs[1] == "run"
}

func run(arguments []string, output io.Writer) error {
	if isServerCommand(arguments) {
		return runServer(arguments, output)
	}
	return runClient(arguments, output)
}

func sharedDeps(output io.Writer) (sharedDepsResult, error) {
	writer := appcli.NewLineWriter(output)
	projectTree := sharedfs.NewOSProjectTree()
	matcherFactory := sharedfs.IgnoreMatcherBuilder(sharedfs.NewGitIgnoreMatcherFactory())
	fileReader := sharedfs.NewOSFileReader()
	indexRepo := idxstorage.NewBinaryIndexRepository()
	checksumRepo := idxstorage.NewDirectoryChecksumRepository()

	configRepo := sharedconfig.NewYAMLRepository()
	cwd, _ := os.Getwd()
	projectRoot := gitRootFrom(cwd)
	cfg, overrides, _ := configRepo.Load(projectRoot)
	configFilePath := configRepo.FilePath(projectRoot)

	matcherFactory = sharedfs.NewCompositeIgnoreMatcherFactory(
		matcherFactory,
		sharedfs.NewGlobIgnoreMatcherFactory(cfg.Index.Ignore),
	)

	return sharedDepsResult{
		writer:         writer,
		projectTree:    projectTree,
		matcherFactory: matcherFactory,
		fileReader:     fileReader,
		indexRepo:      indexRepo,
		checksumRepo:   checksumRepo,
		projectRoot:    projectRoot,
		cfg:            cfg,
		overrides:      overrides,
		configFilePath: configFilePath,
	}, nil
}

type sharedDepsResult struct {
	writer         *appcli.LineWriter
	projectTree    sharedfs.ProjectTree
	matcherFactory sharedfs.IgnoreMatcherBuilder
	fileReader     sharedfs.FileReader
	indexRepo      *idxstorage.BinaryIndexRepository
	checksumRepo   *idxstorage.DirectoryChecksumRepository
	projectRoot    string
	cfg            sharedconfig.IdxConfig
	overrides      []string
	configFilePath string
}

func runServer(arguments []string, output io.Writer) error {
	d, err := sharedDeps(output)
	if err != nil {
		return err
	}

	indexer := featindexing.NewBM25IndexService()
	inspectRunner := idxtui.NewInspectRunner()
	progressRunner := idxtui.NewInitProgressRunner()

	// ServerDaemonService with nil spawner: the server process never spawns itself.
	serverDaemon := featdaemon.NewServerDaemonService(
		featdaemon.NewServerStateRepository(), nil, d.writer,
	)
	initCommand := featindexing.NewInitCommandServiceWithProgress(featindexing.InitCommandServiceDeps{
		ProjectTree:    d.projectTree,
		MatcherFactory: d.matcherFactory,
		Output:         d.writer,
		FileReader:     d.fileReader,
		Indexer:        indexer,
		IndexRepo:      d.indexRepo,
		ChecksumRepo:   d.checksumRepo,
		DaemonRepo:     serverDaemon,
	}, inspectRunner, progressRunner)

	destroyCommand := featlifecycle.NewDestroyCommandService(d.projectTree, d.writer)
	readLogRepo := featread.NewReadLogRepository()
	tuning := featsearch.SearchServiceOptions{
		BM25K1:           d.cfg.BM25.K1,
		BM25B:            d.cfg.BM25.B,
		ProximityWeight:  d.cfg.BM25.ProximityWeight,
		PopularityWeight: d.cfg.BM25.PopularityWeight,
		MaxWorkers:       d.cfg.Search.MaxWorkers,
		CacheTTL:         d.cfg.Search.CacheTTL,
	}
	searchCommand := featsearch.NewSearchCommandService(d.projectTree, d.writer, d.fileReader, d.indexRepo).
		WithTuning(tuning).
		WithReadLog(readLogRepo)
	skillsInstaller := featskills.NewEmbedSkillsInstaller()
	skillsService := featskills.NewSkillsInstallService(skillsInstaller, output)
	fileStreamer := featread.NewOSFileStreamer()
	readService := featread.NewReadCommandService(d.projectTree, fileStreamer, d.writer).
		WithReadLog(readLogRepo)

	socketPath := idxipc.SocketPath(d.projectRoot)
	indexServer := appserver.NewServer(appserver.ServerDeps{
		ProjectTree:    d.projectTree,
		MatcherFactory: d.matcherFactory,
		FileReader:     d.fileReader,
		Indexer:        indexer,
		IndexRepo:      d.indexRepo,
		ChecksumRepo:   d.checksumRepo,
		DaemonRepo:     serverDaemon,
		ReadLogRepo:    readLogRepo,
		SearchTuning:   tuning,
		SocketPath:     socketPath,
	})

	runner := appcli.NewCommandRunner(arguments, initCommand, destroyCommand, searchCommand).
		WithBuildInfo(appcli.BuildInfo{Version: version, BuildDate: buildDate}).
		WithQuietToggle(multiQuiet{d.writer, progressRunner}).
		WithSkillsCommand(skillsService).
		WithReadCommand(readService).
		WithIndexServer(indexServer).
		WithConfig(d.cfg, d.configFilePath, d.overrides)

	return runner.Run()
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

	destroyCommand := featlifecycle.NewDestroyCommandService(d.projectTree, d.writer)
	skillsInstaller := featskills.NewEmbedSkillsInstaller()
	skillsService := featskills.NewSkillsInstallService(skillsInstaller, output)

	socketPath := idxipc.SocketPath(d.projectRoot)
	client := remote.NewSocketClient(socketPath)
	searchCommand := remote.NewRemoteSearcher(client, d.writer)
	readService := remote.NewRemoteReader(client, d.writer)
	indexCmd := remote.NewRemoteIndexCommand(client, d.writer)
	serverDaemonAdapter := appcli.NewServerDaemonAdapter(serverDaemon)

	runner := appcli.NewCommandRunner(arguments, indexCmd, destroyCommand, searchCommand).
		WithBuildInfo(appcli.BuildInfo{Version: version, BuildDate: buildDate}).
		WithQuietToggle(multiQuiet{d.writer, progressRunner}).
		WithSkillsCommand(skillsService).
		WithReadCommand(readService).
		WithServerManager(serverDaemonAdapter).
		WithConfig(d.cfg, d.configFilePath, d.overrides)

	return runner.Run()
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
