package indexing

import (
	"context"
	"fmt"
	"idx/internal/shared/filesystem"
	"idx/internal/shared/output"
)

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

func (service InitCommandService) validateDependencies() error {
	return validateInitDeps(service)
}

func validateInitDeps(s InitCommandService) error {
	type dep struct {
		v          any
		name, want string
	}
	for _, d := range []dep{
		{s.projectTree, "projectTree", "filesystem.ProjectTree"},
		{s.matcherFactory, "matcherFactory", "filesystem.IgnoreMatcherBuilder"},
		{s.output, "output", "output.Writer"},
		{s.fileReader, "fileReader", "filesystem.FileReader"},
		{s.indexer, "indexer", "Indexer"},
		{s.indexRepo, "index repository", "IndexRepository"},
		{s.checksumRepo, "checksum repository", "DirectoryChecksumRepository"},
		{s.inspectUI, "inspect UI", "InspectUIRunner"},
	} {
		if d.v == nil {
			return fmt.Errorf("failed to run init command: got nil %s dependency, expected non-nil %s", d.name, d.want)
		}
	}
	return nil
}
