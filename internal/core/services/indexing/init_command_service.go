package indexing

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"idx/internal/core/domain"
	"idx/internal/core/ports"
)

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

func (service InitCommandService) Sync() error {
	if err := service.validateDependencies(); err != nil {
		return err
	}

	projectRoot, matcher, staleDirectories, eligibleDirectories, err := service.syncPlan()
	if err != nil {
		return err
	}

	if err := service.removeStaleDirectories(staleDirectories); err != nil {
		return err
	}

	if err := service.syncEligibleDirectories(eligibleDirectories, projectRoot, matcher); err != nil {
		return err
	}

	return service.output.WriteLine("✅ Project indices synchronized.")
}

func (service InitCommandService) syncPlan() (string, ports.IgnoreMatcher, []string, []string, error) {
	currentDir, err := service.projectTree.CurrentDir()
	if err != nil {
		return "", nil, nil, nil, fmt.Errorf("failed to resolve current directory: got error %v, expected a readable working directory", err)
	}

	projectRoot, err := service.projectTree.FindGitRoot(currentDir)
	if err != nil {
		return "", nil, nil, nil, err
	}

	if filepath.Clean(currentDir) != filepath.Clean(projectRoot) {
		return "", nil, nil, nil, fmt.Errorf("sync must run from project root: got current directory %q, expected root directory %q", currentDir, projectRoot)
	}

	matcher, err := service.syncMatcher(projectRoot)
	if err != nil {
		return "", nil, nil, nil, err
	}

	directories, err := IndexedDirectories(service.projectTree, projectRoot)
	if err != nil {
		return "", nil, nil, nil, err
	}

	eligibleDirectories, err := eligibleDirectories(service.projectTree, projectRoot, matcher)
	if err != nil {
		return "", nil, nil, nil, err
	}

	staleDirectories := staleIndexedDirectories(directories, eligibleDirectories)
	return projectRoot, matcher, staleDirectories, eligibleDirectories, nil
}

func (service InitCommandService) syncMatcher(projectRoot string) (ports.IgnoreMatcher, error) {
	rootIndexPath := indexFilePath(projectRoot)
	hasRootIndex, err := service.projectTree.Exists(rootIndexPath)
	if err != nil {
		return nil, err
	}

	if !hasRootIndex {
		return nil, fmt.Errorf("sync requires project root to be indexed: no index found at %q, run idx init first", rootIndexPath)
	}

	matcher, err := service.matcherFactory.New(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to load ignore rules for %q: got error %v, expected a readable .gitignore configuration", projectRoot, err)
	}

	return matcher, nil
}

func (service InitCommandService) removeStaleDirectories(staleDirectories []string) error {
	for _, directoryPath := range staleDirectories {
		if err := service.removeDirectoryIndex(directoryPath); err != nil {
			return err
		}
	}

	return nil
}

func (service InitCommandService) syncEligibleDirectories(directories []string, projectRoot string, matcher ports.IgnoreMatcher) error {
	for _, directoryPath := range directories {
		if err := service.syncDirectoryIndex(directoryPath, projectRoot, matcher); err != nil {
			return err
		}
	}

	return nil
}

func (service InitCommandService) removeDirectoryIndex(directoryPath string) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}

	indexDirectoryPath := filepath.Join(directoryPath, ".idx")
	if err := service.projectTree.RemoveAll(indexDirectoryPath); err != nil {
		return err
	}

	return nil
}

func staleIndexedDirectories(indexed []string, eligible []string) []string {
	eligibleSet := make(map[string]struct{}, len(eligible))
	for _, directoryPath := range eligible {
		eligibleSet[directoryPath] = struct{}{}
	}

	stale := make([]string, 0)
	for _, directoryPath := range indexed {
		if _, ok := eligibleSet[directoryPath]; ok {
			continue
		}

		stale = append(stale, directoryPath)
	}

	sort.Strings(stale)
	return stale
}

func (service InitCommandService) Inspect(indexPath string) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}

	currentDir, err := service.projectTree.CurrentDir()
	if err != nil {
		return fmt.Errorf("failed to resolve current directory: got error %v, expected a readable working directory", err)
	}

	trimmedIndexPath := strings.TrimSpace(indexPath)
	if trimmedIndexPath == "" {
		return service.inspectAllProjectIndices(currentDir)
	}

	targetDirectory := filepath.Clean(filepath.Join(currentDir, filepath.FromSlash(path.Clean(trimmedIndexPath))))

	targetIndexPath := indexFilePath(targetDirectory)
	hasIndex, err := service.projectTree.Exists(targetIndexPath)
	if err != nil {
		return err
	}

	if !hasIndex {
		return fmt.Errorf("no index found at %q: run idx init first", targetIndexPath)
	}

	return service.writeInspectIndex(targetDirectory)
}

func (service InitCommandService) inspectAllProjectIndices(currentDir string) error {
	projectRoot, err := service.projectTree.FindGitRoot(currentDir)
	if err != nil {
		return err
	}

	directories, err := IndexedDirectories(service.projectTree, projectRoot)
	if err != nil {
		return err
	}

	if len(directories) == 0 {
		return fmt.Errorf("no index found under project root %q: run idx init first", projectRoot)
	}

	return service.runInspectTUIForDirectories(directories)
}

func (service InitCommandService) runInspectTUIForDirectory(directoryPath string) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}

	index, err := service.indexRepo.LoadIndex(directoryPath)
	if err != nil {
		return err
	}

	return service.inspectUI.Run(index)
}

func (service InitCommandService) runInspectTUIForDirectories(directoryPaths []string) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}

	mergedIndex, err := service.loadMergedInspectIndex(directoryPaths)
	if err != nil {
		return err
	}

	return service.inspectUI.Run(mergedIndex)
}

func (service InitCommandService) loadMergedInspectIndex(directoryPaths []string) (*domain.InvertedIndex, error) {
	sortedDirectories := append([]string(nil), directoryPaths...)
	sort.Strings(sortedDirectories)

	indicesByDirectory := make(map[string]*domain.InvertedIndex, len(sortedDirectories))
	for _, directoryPath := range sortedDirectories {
		index, err := service.indexRepo.LoadIndex(directoryPath)
		if err != nil {
			return nil, err
		}

		indicesByDirectory[directoryPath] = index
	}

	return mergeInspectIndices(indicesByDirectory), nil
}

func mergeInspectIndices(indicesByDirectory map[string]*domain.InvertedIndex) *domain.InvertedIndex {
	merged := domain.NewInvertedIndex()
	for _, directoryPath := range sortedIndexDirectories(indicesByDirectory) {
		mergeInspectIndexDirectory(merged, directoryPath, indicesByDirectory[directoryPath])
	}

	merged.DocumentCount = len(merged.Documents)
	merged.CalculateAverageDocLen()
	merged.CalculateIDF()
	return merged
}

func sortedIndexDirectories(indicesByDirectory map[string]*domain.InvertedIndex) []string {
	directories := make([]string, 0, len(indicesByDirectory))
	for directoryPath := range indicesByDirectory {
		directories = append(directories, directoryPath)
	}

	sort.Strings(directories)
	return directories
}

func mergeInspectIndexDirectory(target *domain.InvertedIndex, directoryPath string, index *domain.InvertedIndex) {
	if index == nil {
		return
	}

	mergeInspectDocuments(target, directoryPath, index)
	mergeInspectTerms(target, directoryPath, index)
}

func mergeInspectDocuments(target *domain.InvertedIndex, directoryPath string, index *domain.InvertedIndex) {
	for documentName, stats := range index.Documents {
		if stats == nil {
			continue
		}

		documentID := inspectDocumentID(directoryPath, documentName)
		target.Documents[documentID] = &domain.DocStats{
			Name:   stats.Name,
			Path:   stats.Path,
			Length: stats.Length,
		}
	}
}

func mergeInspectTerms(target *domain.InvertedIndex, directoryPath string, index *domain.InvertedIndex) {
	for term, termStats := range index.Terms {
		if termStats == nil {
			continue
		}

		targetTerm := target.Terms[term]
		if targetTerm == nil {
			targetTerm = &domain.TermStats{Docs: make(map[string]*domain.DocTermStats)}
			target.Terms[term] = targetTerm
		}

		for documentName, docTermStats := range termStats.Docs {
			documentID := inspectDocumentID(directoryPath, documentName)
			targetTerm.Docs[documentID] = cloneInspectDocTermStats(docTermStats)
		}
	}
}

func cloneInspectDocTermStats(docTermStats *domain.DocTermStats) *domain.DocTermStats {
	if docTermStats == nil {
		return &domain.DocTermStats{}
	}

	positions := append([]int(nil), docTermStats.Positions...)
	return &domain.DocTermStats{TF: docTermStats.TF, Positions: positions}
}

func inspectDocumentID(directoryPath string, documentName string) string {
	return directoryPath + "::" + documentName
}

func (service InitCommandService) Watch(showUpdatedFiles bool, debounce time.Duration) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}

	if debounce <= 0 {
		return fmt.Errorf("failed to run watch command: got invalid debounce %s, expected duration greater than 0", debounce)
	}

	// Check if daemon is running for this project
	if service.daemonRepo != nil {
		state, _ := service.daemonRepo.ReadState()
		if state != nil && len(state.Projects) > 0 {
			return fmt.Errorf("cannot run watch: daemon is already monitoring this project. Disable the daemon with 'idx daemon disable' first")
		}
	}

	return service.watchLoop(showUpdatedFiles, debounce)
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

func (service InitCommandService) writeInspectIndex(directoryPath string) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}

	index, err := service.indexRepo.LoadIndex(directoryPath)
	if err != nil {
		return err
	}

	encoded, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode debug index for %q: got error %v, expected valid index payload", directoryPath, err)
	}

	return service.output.WriteLine(string(encoded))
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
