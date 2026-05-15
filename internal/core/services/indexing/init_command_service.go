package indexing

import (
	"context"
	"fmt"
	"os"
	"time"

	"idx/internal/core/domain"
	"idx/internal/core/ports"
)

const daemonChildEnvVar = "IDX_DAEMON_CHILD"

type InitCommandService struct {
	projectTree           ports.ProjectTree
	matcherFactory        ports.IgnoreMatcherFactory
	output                ports.TextOutput
	fileReader            ports.FileReader
	indexer               ports.BM25Indexer
	indexRepo             ports.IndexRepository
	checksumRepo          ports.DirectoryChecksumRepository
	daemonRepo            ports.DaemonRepository
	inspectUI             ports.InspectUIRunner
	initProgress          ports.InitProgress
	statusConfigFilePath  string
	statusConfigOverrides []string
}

// disabledInspectUIRunner is the default when no TUI adapter is injected.
// Returns an error instead of panicking to signal misconfiguration early.
type disabledInspectUIRunner struct{}

func (disabledInspectUIRunner) Run(_ *domain.InvertedIndex) error {
	return fmt.Errorf("inspect UI not configured: use NewInitCommandServiceWithInspectUI to enable the TUI")
}

type disabledInitProgress struct{}

func (disabledInitProgress) StartCounting()           {}
func (disabledInitProgress) SetTotal(int)             {}
func (disabledInitProgress) IncrementDir(string)      {}
func (disabledInitProgress) Finish()                  {}
func (disabledInitProgress) Context() context.Context { return context.Background() }

// NewInitCommandService builds the init use case without TUI support.
// Use NewInitCommandServiceWithInspectUI when the inspect command must launch the TUI.
// Example: service := NewInitCommandService(projectTree, matcherFactory, output, fileReader, indexer, indexRepo, checksumRepo, daemonRepo).
func NewInitCommandService(projectTree ports.ProjectTree, matcherFactory ports.IgnoreMatcherFactory, output ports.TextOutput, fileReader ports.FileReader, indexer ports.BM25Indexer, indexRepo ports.IndexRepository, checksumRepo ports.DirectoryChecksumRepository, daemonRepo ports.DaemonRepository) InitCommandService {
	return NewInitCommandServiceWithInspectUI(projectTree, matcherFactory, output, fileReader, indexer, indexRepo, checksumRepo, daemonRepo, disabledInspectUIRunner{})
}

// NewInitCommandServiceWithInspectUI builds the init use case with an injected inspect UI runner.
// Example: service := NewInitCommandServiceWithInspectUI(projectTree, matcherFactory, output, fileReader, indexer, indexRepo, checksumRepo, daemonRepo, inspectUI).
func NewInitCommandServiceWithInspectUI(projectTree ports.ProjectTree, matcherFactory ports.IgnoreMatcherFactory, output ports.TextOutput, fileReader ports.FileReader, indexer ports.BM25Indexer, indexRepo ports.IndexRepository, checksumRepo ports.DirectoryChecksumRepository, daemonRepo ports.DaemonRepository, inspectUI ports.InspectUIRunner) InitCommandService {
	return NewInitCommandServiceWithProgress(projectTree, matcherFactory, output, fileReader, indexer, indexRepo, checksumRepo, daemonRepo, inspectUI, disabledInitProgress{})
}

// NewInitCommandServiceWithProgress builds the init use case with both inspect UI and a progress reporter.
// Example: service := NewInitCommandServiceWithProgress(..., inspectUI, progressRunner).
func NewInitCommandServiceWithProgress(projectTree ports.ProjectTree, matcherFactory ports.IgnoreMatcherFactory, output ports.TextOutput, fileReader ports.FileReader, indexer ports.BM25Indexer, indexRepo ports.IndexRepository, checksumRepo ports.DirectoryChecksumRepository, daemonRepo ports.DaemonRepository, inspectUI ports.InspectUIRunner, progress ports.InitProgress) InitCommandService {
	return InitCommandService{
		projectTree:    projectTree,
		matcherFactory: matcherFactory,
		output:         output,
		fileReader:     fileReader,
		indexer:        indexer,
		indexRepo:      indexRepo,
		checksumRepo:   checksumRepo,
		daemonRepo:     daemonRepo,
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
		return fmt.Errorf("cannot run watch: daemon is already monitoring this project. Disable the daemon with 'idx daemon disable' first")
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
	state, _ := service.daemonRepo.ReadState()
	return isProjectInDaemonState(state, projectRoot), nil
}

func (service InitCommandService) currentProjectRoot() (string, error) {
	currentDir, err := service.projectTree.CurrentDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve current directory: got error %v, expected a readable working directory", err)
	}
	return service.projectTree.FindGitRoot(currentDir)
}

func isProjectInDaemonState(state *domain.DaemonState, projectRoot string) bool {
	if state == nil {
		return false
	}
	for _, project := range state.Projects {
		if project.Enabled && project.Path == projectRoot {
			return true
		}
	}
	return false
}

func (service InitCommandService) validateDependencies() error {
	if service.projectTree == nil {
		return fmt.Errorf("failed to run init command: got nil projectTree dependency, expected non-nil ports.ProjectTree")
	}

	if service.matcherFactory == nil {
		return fmt.Errorf("failed to run init command: got nil matcherFactory dependency, expected non-nil ports.IgnoreMatcherFactory")
	}

	if service.output == nil {
		return fmt.Errorf("failed to run init command: got nil output dependency, expected non-nil ports.TextOutput")
	}

	if service.fileReader == nil {
		return fmt.Errorf("failed to run init command: got nil fileReader dependency, expected non-nil ports.FileReader")
	}

	if service.indexer == nil {
		return fmt.Errorf("failed to run init command: got nil indexer dependency, expected non-nil ports.BM25Indexer")
	}

	if service.indexRepo == nil {
		return fmt.Errorf("failed to run init command: got nil index repository dependency, expected non-nil ports.IndexRepository")
	}

	if service.checksumRepo == nil {
		return fmt.Errorf("failed to run init command: got nil checksum repository dependency, expected non-nil ports.DirectoryChecksumRepository")
	}

	if service.inspectUI == nil {
		return fmt.Errorf("failed to run init command: got nil inspect UI dependency, expected non-nil ports.InspectUIRunner")
	}

	return nil
}

