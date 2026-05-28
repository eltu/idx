package indexing

import (
	"context"
	"fmt"
	"idx/internal/shared/filesystem"
	"idx/internal/shared/output"
	"os"
	"time"
)

const daemonChildEnvVar = "IDX_SERVER_DAEMON"

type InitCommandService struct {
	projectTree           filesystem.ProjectTree
	matcherFactory        filesystem.IgnoreMatcherBuilder
	output                output.Writer
	fileReader            filesystem.FileReader
	indexer               Indexer
	indexRepo             IndexRepository
	checksumRepo          DirectoryChecksumRepository
	daemonRepo            ProjectMonitorChecker
	inspectUI             InspectUIRunner
	initProgress          Progress
	statusConfigFilePath  string
	statusConfigOverrides []string
}

// disabledInspectUIRunner is the default when no TUI adapter is injected.
// Returns an error instead of panicking to signal misconfiguration early.
type disabledInspectUIRunner struct{}

func (disabledInspectUIRunner) Run(_ *InvertedIndex) error {
	return fmt.Errorf("inspect UI not configured: use NewInitCommandServiceWithInspectUI to enable the TUI")
}

type disabledInitProgress struct{}

func (disabledInitProgress) StartCounting()           { /* no-op: progress reporting disabled */ }
func (disabledInitProgress) SetTotal(int)             { /* no-op: progress reporting disabled */ }
func (disabledInitProgress) IncrementDir(string)      { /* no-op: progress reporting disabled */ }
func (disabledInitProgress) Finish()                  { /* no-op: progress reporting disabled */ }
func (disabledInitProgress) Context() context.Context { return context.Background() }

// InitCommandServiceDeps groups all collaborators for InitCommandService constructors,
// eliminating the SonarQube too-many-parameters violation on the three New* functions.
type InitCommandServiceDeps struct {
	ProjectTree    filesystem.ProjectTree
	MatcherFactory filesystem.IgnoreMatcherBuilder
	Output         output.Writer
	FileReader     filesystem.FileReader
	Indexer        Indexer
	IndexRepo      IndexRepository
	ChecksumRepo   DirectoryChecksumRepository
	DaemonRepo     ProjectMonitorChecker
}

// NewInitCommandService builds the init use case without TUI support.
// Use NewInitCommandServiceWithInspectUI when the inspect command must launch the TUI.
// Example: service := NewInitCommandService(indexing.InitCommandServiceDeps{...}).
func NewInitCommandService(deps InitCommandServiceDeps) InitCommandService {
	return NewInitCommandServiceWithInspectUI(deps, disabledInspectUIRunner{})
}

// NewInitCommandServiceWithInspectUI builds the init use case with an injected inspect UI runner.
// Example: service := NewInitCommandServiceWithInspectUI(indexing.InitCommandServiceDeps{...}, inspectUI).
func NewInitCommandServiceWithInspectUI(deps InitCommandServiceDeps, inspectUI InspectUIRunner) InitCommandService {
	return NewInitCommandServiceWithProgress(deps, inspectUI, disabledInitProgress{})
}

// NewInitCommandServiceWithProgress builds the init use case with both inspect UI and a progress reporter.
// Example: service := NewInitCommandServiceWithProgress(indexing.InitCommandServiceDeps{...}, inspectUI, progressRunner).
func NewInitCommandServiceWithProgress(deps InitCommandServiceDeps, inspectUI InspectUIRunner, progress Progress) InitCommandService {
	return InitCommandService{
		projectTree:    deps.ProjectTree,
		matcherFactory: deps.MatcherFactory,
		output:         deps.Output,
		fileReader:     deps.FileReader,
		indexer:        deps.Indexer,
		indexRepo:      deps.IndexRepo,
		checksumRepo:   deps.ChecksumRepo,
		daemonRepo:     deps.DaemonRepo,
		inspectUI:      inspectUI,
		initProgress:   progress,
	}
}

func (service InitCommandService) Run() error {
	if err := service.validateDependencies(); err != nil {
		return err
	}

	return service.runIndex()
}

func (service InitCommandService) Watch(showUpdatedFiles bool, debounce time.Duration) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}

	if debounce <= 0 {
		return fmt.Errorf("failed to run watch command: got invalid debounce %s, expected duration greater than 0", debounce)
	}

	if service.watchStartedByDaemon() {
		return service.watchLoop(showUpdatedFiles, debounce)
	}

	monitored, err := service.currentProjectAlreadyMonitored()
	if err != nil {
		return err
	}
	if monitored {
		return fmt.Errorf("cannot run watch: server is already monitoring this project. Stop it with 'idx server stop' first")
	}

	return service.watchLoop(showUpdatedFiles, debounce)
}

func (service InitCommandService) watchStartedByDaemon() bool {
	return os.Getenv(daemonChildEnvVar) == "1"
}

func (service InitCommandService) currentProjectAlreadyMonitored() (bool, error) {
	if service.daemonRepo == nil {
		return false, nil
	}
	projectRoot, err := service.currentProjectRoot()
	if err != nil {
		return false, err
	}
	return service.daemonRepo.IsProjectMonitored(projectRoot)
}

func (service InitCommandService) currentProjectRoot() (string, error) {
	currentDir, err := service.projectTree.CurrentDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve current directory: got error %v, expected a readable working directory", err)
	}
	return service.projectTree.FindGitRoot(currentDir)
}

func (service InitCommandService) validateDependencies() error {
	if service.projectTree == nil {
		return fmt.Errorf("failed to run init command: got nil projectTree dependency, expected non-nil filesystem.ProjectTree")
	}

	if service.matcherFactory == nil {
		return fmt.Errorf("failed to run init command: got nil matcherFactory dependency, expected non-nil filesystem.IgnoreMatcherBuilder")
	}

	if service.output == nil {
		return fmt.Errorf("failed to run init command: got nil output dependency, expected non-nil output.Writer")
	}

	if service.fileReader == nil {
		return fmt.Errorf("failed to run init command: got nil fileReader dependency, expected non-nil filesystem.FileReader")
	}

	if service.indexer == nil {
		return fmt.Errorf("failed to run init command: got nil indexer dependency, expected non-nil Indexer")
	}

	if service.indexRepo == nil {
		return fmt.Errorf("failed to run init command: got nil index repository dependency, expected non-nil IndexRepository")
	}

	if service.checksumRepo == nil {
		return fmt.Errorf("failed to run init command: got nil checksum repository dependency, expected non-nil DirectoryChecksumRepository")
	}

	if service.inspectUI == nil {
		return fmt.Errorf("failed to run init command: got nil inspect UI dependency, expected non-nil InspectUIRunner")
	}

	return nil
}
