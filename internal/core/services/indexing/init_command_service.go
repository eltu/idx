package indexing

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"idx/internal/core/domain"
	"idx/internal/core/ports"
)

const daemonChildEnvVar = "IDX_DAEMON_CHILD"

type InitCommandService struct {
	projectTree    ports.ProjectTree
	matcherFactory ports.IgnoreMatcherFactory
	output         ports.TextOutput
	fileReader     ports.FileReader
	indexer        ports.BM25Indexer
	indexRepo      ports.IndexRepository
	checksumRepo   ports.DirectoryChecksumRepository
	daemonRepo     ports.DaemonRepository
	inspectUI      ports.InspectUIRunner
}

// NewInitCommandService builds the init use case.
// Example: service := NewInitCommandService(projectTree, matcherFactory, output, fileReader, indexer, indexRepo, checksumRepo, daemonRepo).
func NewInitCommandService(projectTree ports.ProjectTree, matcherFactory ports.IgnoreMatcherFactory, output ports.TextOutput, fileReader ports.FileReader, indexer ports.BM25Indexer, indexRepo ports.IndexRepository, checksumRepo ports.DirectoryChecksumRepository, daemonRepo ports.DaemonRepository) InitCommandService {
	return NewInitCommandServiceWithInspectUI(projectTree, matcherFactory, output, fileReader, indexer, indexRepo, checksumRepo, daemonRepo, defaultInspectUIRunner{})
}

// NewInitCommandServiceWithInspectUI builds the init use case with an injected inspect UI runner.
// Example: service := NewInitCommandServiceWithInspectUI(projectTree, matcherFactory, output, fileReader, indexer, indexRepo, checksumRepo, daemonRepo, inspectUI).
func NewInitCommandServiceWithInspectUI(projectTree ports.ProjectTree, matcherFactory ports.IgnoreMatcherFactory, output ports.TextOutput, fileReader ports.FileReader, indexer ports.BM25Indexer, indexRepo ports.IndexRepository, checksumRepo ports.DirectoryChecksumRepository, daemonRepo ports.DaemonRepository, inspectUI ports.InspectUIRunner) InitCommandService {
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

	currentDir, err := service.projectTree.CurrentDir()
	if err != nil {
		return false, fmt.Errorf("failed to resolve current directory: got error %v, expected a readable working directory", err)
	}

	projectRoot, err := service.projectTree.FindGitRoot(currentDir)
	if err != nil {
		return false, err
	}

	state, _ := service.daemonRepo.ReadState()
	if state == nil {
		return false, nil
	}

	for _, project := range state.Projects {
		if project.Enabled && project.Path == projectRoot {
			return true, nil
		}
	}

	return false, nil
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

func (service InitCommandService) runIndex() error {
	currentDir, err := service.projectTree.CurrentDir()
	if err != nil {
		return fmt.Errorf("failed to resolve current directory: got error %v, expected a readable working directory", err)
	}

	projectRoot, err := service.projectTree.FindGitRoot(currentDir)
	if err != nil {
		return err
	}

	if err := service.ensureIdxRuleInGitIgnore(projectRoot); err != nil {
		return err
	}

	currentIndexPath := indexFilePath(currentDir)
	hasIndex, err := service.projectTree.Exists(currentIndexPath)
	if err != nil {
		return err
	}

	if hasIndex {
		return service.output.WriteLine("ℹ️ This project is already indexed. You can run idx search.")
	}

	matcher, err := service.matcherFactory.New(projectRoot)
	if err != nil {
		return fmt.Errorf("failed to load ignore rules for %q: got error %v, expected a readable .gitignore configuration", projectRoot, err)
	}

	if err := service.indexDirectory(currentDir, projectRoot, matcher); err != nil {
		return err
	}

	return service.output.WriteLine("✅ Index created. You can now run idx search.")
}

func (service InitCommandService) ensureIdxRuleInGitIgnore(projectRoot string) error {
	gitIgnorePath := filepath.Join(projectRoot, ".gitignore")
	content, err := service.fileReader.ReadFile(gitIgnorePath)
	if err != nil {
		if !isMissingFileError(err) {
			return fmt.Errorf("failed to read project .gitignore %q: got error %v, expected readable file", gitIgnorePath, err)
		}

		return service.projectTree.WriteFile(gitIgnorePath, []byte(".idx/\n"))
	}

	if hasIdxDirectoryIgnoreRule(content) {
		return nil
	}

	updated := appendIdxDirectoryIgnoreRule(content)
	return service.projectTree.WriteFile(gitIgnorePath, []byte(updated))
}

func hasIdxDirectoryIgnoreRule(content string) bool {
	lines := strings.Split(content, "\n")
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}

		normalized := normalizeIgnorePattern(line)
		if normalized == ".idx" {
			return true
		}
	}

	return false
}

func normalizeIgnorePattern(pattern string) string {
	normalized := strings.TrimSpace(pattern)
	normalized = strings.TrimPrefix(normalized, "/")
	normalized = strings.TrimSuffix(normalized, "/")
	normalized = strings.TrimSuffix(normalized, "/**")
	normalized = strings.TrimPrefix(normalized, "**/")
	return normalized
}

func appendIdxDirectoryIgnoreRule(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ".idx/\n"
	}

	if strings.HasSuffix(content, "\n") {
		return content + ".idx/\n"
	}

	return content + "\n.idx/\n"
}

func isMissingFileError(err error) bool {
	if os.IsNotExist(err) {
		return true
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "file not found") || strings.Contains(message, "no such file or directory")
}

func (service InitCommandService) indexDirectory(directoryPath string, projectRoot string, matcher ports.IgnoreMatcher) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}

	if err := service.syncDirectoryIndex(directoryPath, projectRoot, matcher); err != nil {
		return err
	}

	entries, err := service.projectTree.ReadDir(directoryPath)
	if err != nil {
		return fmt.Errorf("failed to read directory %q: got error %v, expected a readable directory", directoryPath, err)
	}

	allowedEntries, err := filterEntries(entries, projectRoot, matcher)
	if err != nil {
		return err
	}

	dirEntries := make([]domain.DirectoryEntry, 0)
	for _, entry := range allowedEntries {
		if entry.IsDir {
			dirEntries = append(dirEntries, entry)
		}
	}

	// Recursively index subdirectories
	return service.indexChildren(dirEntries, projectRoot, matcher)
}

func (service InitCommandService) syncDirectoryIndex(directoryPath string, projectRoot string, matcher ports.IgnoreMatcher) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}

	fileEntries, err := service.indexableFileEntries(directoryPath, projectRoot, matcher)
	if err != nil {
		return err
	}

	if len(fileEntries) == 0 {
		return service.removeDirectoryIndex(directoryPath)
	}

	currentSnapshot, changedFileNames, shouldReindex, err := service.reindexState(directoryPath, fileEntries)
	if err != nil {
		return err
	}

	if !shouldReindex {
		return nil
	}

	if err := service.buildAndSaveIndex(directoryPath, fileEntries, currentSnapshot, changedFileNames); err != nil {
		return err
	}

	return service.saveChecksumSnapshot(directoryPath, currentSnapshot)
}

func (service InitCommandService) indexableFileEntries(directoryPath string, projectRoot string, matcher ports.IgnoreMatcher) ([]domain.DirectoryEntry, error) {
	entries, err := service.projectTree.ReadDir(directoryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %q: got error %v, expected a readable directory", directoryPath, err)
	}

	allowedEntries, err := filterEntries(entries, projectRoot, matcher)
	if err != nil {
		return nil, err
	}

	return filesFromEntries(allowedEntries), nil
}

func (service InitCommandService) reindexState(directoryPath string, fileEntries []domain.DirectoryEntry) (ports.DirectoryChecksumSnapshot, map[string]struct{}, bool, error) {

	hasIndex, err := service.hasDirectoryIndex(directoryPath)
	if err != nil {
		return ports.DirectoryChecksumSnapshot{}, nil, false, err
	}

	storedSnapshot, hasSnapshot, err := service.loadChecksumSnapshot(directoryPath)
	if err != nil {
		return ports.DirectoryChecksumSnapshot{}, nil, false, err
	}

	currentSnapshot, changed, changedFileNames, err := service.computeDirectorySnapshot(fileEntries, storedSnapshot)
	if err != nil {
		return ports.DirectoryChecksumSnapshot{}, nil, false, err
	}

	shouldReindex := !hasIndex || !hasSnapshot || changed
	return currentSnapshot, changedFileNames, shouldReindex, nil
}

func (service InitCommandService) buildAndSaveIndex(directoryPath string, fileEntries []domain.DirectoryEntry, snapshot ports.DirectoryChecksumSnapshot, changedFileNames map[string]struct{}) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}

	// Read all files and build index documents.
	documents := make([]domain.IndexDocument, 0, len(fileEntries))
	for _, entry := range fileEntries {
		content, err := service.fileReader.ReadFile(entry.Path)
		if err != nil {
			return err
		}

		documents = append(documents, domain.IndexDocument{
			Name:    entry.Name,
			Path:    entry.Path,
			Content: content,
		})
	}

	// Build BM25 index
	index, err := service.indexer.BuildIndex(documents)
	if err != nil {
		return fmt.Errorf("failed to build BM25 index for %q: got error %v, expected valid document content", directoryPath, err)
	}

	// Save index
	if err := service.indexRepo.SaveIndex(directoryPath, index); err != nil {
		return err
	}

	logEntries := buildChangedLogEntries(fileEntries, snapshot, changedFileNames, time.Now().UTC())

	if err := appendIndexedFilesLog(directoryPath, logEntries); err != nil {
		return err
	}

	return nil
}

func buildChangedLogEntries(fileEntries []domain.DirectoryEntry, snapshot ports.DirectoryChecksumSnapshot, changedFileNames map[string]struct{}, indexedAt time.Time) []indexedFileLogEntry {
	if len(changedFileNames) == 0 {
		return []indexedFileLogEntry{}
	}

	entries := make([]indexedFileLogEntry, 0, len(changedFileNames))
	for _, entry := range fileEntries {
		if _, changed := changedFileNames[entry.Name]; !changed {
			continue
		}

		state, exists := snapshot.Files[entry.Name]
		if !exists || state.Checksum == "" {
			continue
		}

		entries = append(entries, indexedFileLogEntry{
			Path:      entry.Path,
			Checksum:  state.Checksum,
			IndexedAt: indexedAt,
		})
	}

	return entries
}

func (service InitCommandService) indexChildren(entries []domain.DirectoryEntry, projectRoot string, matcher ports.IgnoreMatcher) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir {
			continue
		}

		if err := service.indexDirectory(entry.Path, projectRoot, matcher); err != nil {
			return err
		}
	}

	return nil
}

func indexFilePath(directoryPath string) string {
	return filepath.Join(directoryPath, ".idx", "index.idx")
}
