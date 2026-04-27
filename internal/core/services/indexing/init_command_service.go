package indexing

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"sort"
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
}

// NewInitCommandService builds the init use case.
// Example: service := NewInitCommandService(projectTree, matcherFactory, output, fileReader, indexer, indexRepo).
func NewInitCommandService(projectTree ports.ProjectTree, matcherFactory ports.IgnoreMatcherFactory, output ports.TextOutput, fileReader ports.FileReader, indexer ports.BM25Indexer, indexRepo ports.IndexRepository, checksumRepo ports.DirectoryChecksumRepository) InitCommandService {
	return InitCommandService{
		projectTree:    projectTree,
		matcherFactory: matcherFactory,
		output:         output,
		fileReader:     fileReader,
		indexer:        indexer,
		indexRepo:      indexRepo,
		checksumRepo:   checksumRepo,
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

	currentDir, err := service.projectTree.CurrentDir()
	if err != nil {
		return fmt.Errorf("failed to resolve current directory: got error %v, expected a readable working directory", err)
	}

	projectRoot, err := service.projectTree.FindGitRoot(currentDir)
	if err != nil {
		return err
	}

	if filepath.Clean(currentDir) != filepath.Clean(projectRoot) {
		return fmt.Errorf("sync must run from project root: got current directory %q, expected root directory %q", currentDir, projectRoot)
	}

	rootIndexPath := indexFilePath(projectRoot)
	hasRootIndex, err := service.projectTree.Exists(rootIndexPath)
	if err != nil {
		return err
	}

	if !hasRootIndex {
		return fmt.Errorf("sync requires project root to be indexed: no index found at %q, run idx init first", rootIndexPath)
	}

	matcher, err := service.matcherFactory.New(projectRoot)
	if err != nil {
		return fmt.Errorf("failed to load ignore rules for %q: got error %v, expected a readable .gitignore configuration", projectRoot, err)
	}

	directories, err := IndexedDirectories(service.projectTree, projectRoot)
	if err != nil {
		return err
	}

	eligible, err := eligibleDirectories(service.projectTree, projectRoot, matcher)
	if err != nil {
		return err
	}

	staleDirectories := staleIndexedDirectories(directories, eligible)
	for _, directoryPath := range staleDirectories {
		if err := service.removeDirectoryIndex(directoryPath); err != nil {
			return err
		}
	}

	for _, directoryPath := range eligible {
		if err := service.syncDirectoryIndex(directoryPath, projectRoot, matcher); err != nil {
			return err
		}
	}

	return service.output.WriteLine("✅ Project indices synchronized.")
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

	targetDirectory := filepath.Clean(filepath.Join(currentDir, filepath.FromSlash(path.Clean(indexPath))))
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

	return nil
}

func (service InitCommandService) runIndex() error {
	currentDir, err := service.projectTree.CurrentDir()
	if err != nil {
		return fmt.Errorf("failed to resolve current directory: got error %v, expected a readable working directory", err)
	}

	currentIndexPath := indexFilePath(currentDir)
	hasIndex, err := service.projectTree.Exists(currentIndexPath)
	if err != nil {
		return err
	}

	if hasIndex {
		return service.output.WriteLine("ℹ️ This project is already indexed. You can run idx search.")
	}

	projectRoot, err := service.projectTree.FindGitRoot(currentDir)
	if err != nil {
		return err
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

	entries, err := service.projectTree.ReadDir(directoryPath)
	if err != nil {
		return fmt.Errorf("failed to read directory %q: got error %v, expected a readable directory", directoryPath, err)
	}

	allowedEntries, err := filterEntries(entries, projectRoot, matcher)
	if err != nil {
		return err
	}

	fileEntries := make([]domain.DirectoryEntry, 0)
	for _, entry := range allowedEntries {
		if entry.IsDir {
			continue
		}

		fileEntries = append(fileEntries, entry)
	}

	if len(fileEntries) == 0 {
		return service.removeDirectoryIndex(directoryPath)
	}

	hasIndex, err := service.hasDirectoryIndex(directoryPath)
	if err != nil {
		return err
	}

	storedSnapshot, hasSnapshot, err := service.loadChecksumSnapshot(directoryPath)
	if err != nil {
		return err
	}

	currentSnapshot, changed, changedFileNames, err := service.computeDirectorySnapshot(fileEntries, storedSnapshot)
	if err != nil {
		return err
	}

	shouldReindex := !hasIndex || !hasSnapshot || changed

	if !shouldReindex {
		return nil
	}

	if err := service.buildAndSaveIndex(directoryPath, fileEntries, currentSnapshot, changedFileNames); err != nil {
		return err
	}

	return service.saveChecksumSnapshot(directoryPath, currentSnapshot)
}

func (service InitCommandService) hasDirectoryIndex(directoryPath string) (bool, error) {
	if err := service.validateDependencies(); err != nil {
		return false, err
	}

	currentIndexPath := indexFilePath(directoryPath)
	hasIndex, err := service.projectTree.Exists(currentIndexPath)
	if err != nil {
		return false, fmt.Errorf("failed to check index file %q: got error %v, expected readable filesystem", currentIndexPath, err)
	}

	return hasIndex, nil
}

func (service InitCommandService) loadChecksumSnapshot(directoryPath string) (ports.DirectoryChecksumSnapshot, bool, error) {
	if err := service.validateDependencies(); err != nil {
		return ports.DirectoryChecksumSnapshot{}, false, err
	}

	if repositoryWithSnapshot, ok := service.checksumRepo.(ports.DirectoryChecksumSnapshotRepository); ok {
		return repositoryWithSnapshot.LoadSnapshot(directoryPath)
	}

	checksums, exists, err := service.checksumRepo.Load(directoryPath)
	if err != nil {
		return ports.DirectoryChecksumSnapshot{}, false, err
	}

	files := make(map[string]ports.FileChecksumState, len(checksums))
	for fileName, checksum := range checksums {
		files[fileName] = ports.FileChecksumState{Checksum: checksum}
	}

	return ports.DirectoryChecksumSnapshot{Files: files}, exists, nil
}

func (service InitCommandService) saveChecksumSnapshot(directoryPath string, snapshot ports.DirectoryChecksumSnapshot) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}

	if repositoryWithSnapshot, ok := service.checksumRepo.(ports.DirectoryChecksumSnapshotRepository); ok {
		return repositoryWithSnapshot.SaveSnapshot(directoryPath, snapshot)
	}

	checksums := make(map[string]string, len(snapshot.Files))
	for fileName, state := range snapshot.Files {
		checksums[fileName] = state.Checksum
	}

	return service.checksumRepo.Save(directoryPath, checksums)
}

func (service InitCommandService) computeDirectorySnapshot(fileEntries []domain.DirectoryEntry, stored ports.DirectoryChecksumSnapshot) (ports.DirectoryChecksumSnapshot, bool, map[string]struct{}, error) {
	current := ports.DirectoryChecksumSnapshot{Files: make(map[string]ports.FileChecksumState, len(fileEntries))}
	changed := false
	changedFileNames := make(map[string]struct{})

	if len(stored.Files) != len(fileEntries) {
		changed = true
	}

	for _, entry := range fileEntries {
		storedState, exists := stored.Files[entry.Name]
		if exists && metadataUnchanged(entry, storedState) && storedState.Checksum != "" {
			current.Files[entry.Name] = storedState
			continue
		}

		checksum, err := service.fileChecksum(entry)
		if err != nil {
			return ports.DirectoryChecksumSnapshot{}, false, nil, err
		}

		currentState := ports.FileChecksumState{Checksum: checksum, Size: entry.Size, ModTimeUnixNano: entry.ModTimeUnixNano}
		current.Files[entry.Name] = currentState

		if !exists || storedState.Checksum != checksum {
			changed = true
			changedFileNames[entry.Name] = struct{}{}
		}
	}

	if !changed && !sameSnapshotChecksums(stored.Files, current.Files) {
		changed = true
	}

	return current, changed, changedFileNames, nil
}

func metadataUnchanged(entry domain.DirectoryEntry, stored ports.FileChecksumState) bool {
	if entry.Size == 0 && entry.ModTimeUnixNano == 0 {
		return false
	}

	if stored.ModTimeUnixNano == 0 && stored.Size == 0 {
		return false
	}

	return entry.Size == stored.Size && entry.ModTimeUnixNano == stored.ModTimeUnixNano
}

func (service InitCommandService) fileChecksum(entry domain.DirectoryEntry) (string, error) {
	if err := service.validateDependencies(); err != nil {
		return "", err
	}

	content, err := service.fileReader.ReadFile(entry.Path)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:]), nil
}

func sameSnapshotChecksums(stored map[string]ports.FileChecksumState, current map[string]ports.FileChecksumState) bool {
	if len(stored) != len(current) {
		return false
	}

	for fileName, storedState := range stored {
		currentState, exists := current[fileName]
		if !exists {
			return false
		}

		if storedState.Checksum != currentState.Checksum {
			return false
		}
	}

	return true
}

func (service InitCommandService) shouldReindexDirectory(directoryPath string, currentChecksums map[string]string) (bool, error) {
	if err := service.validateDependencies(); err != nil {
		return false, err
	}

	hasIndex, err := service.hasDirectoryIndex(directoryPath)
	if err != nil {
		return false, err
	}

	if !hasIndex {
		return true, nil
	}

	storedChecksums, exists, err := service.checksumRepo.Load(directoryPath)
	if err != nil {
		return false, err
	}

	if !exists {
		return true, nil
	}

	return !sameChecksums(storedChecksums, currentChecksums), nil
}

func (service InitCommandService) directoryChecksums(fileEntries []domain.DirectoryEntry) (map[string]string, error) {
	if err := service.validateDependencies(); err != nil {
		return nil, err
	}

	checksums := make(map[string]string, len(fileEntries))
	for _, entry := range fileEntries {
		content, err := service.fileReader.ReadFile(entry.Path)
		if err != nil {
			return nil, err
		}

		sum := sha256.Sum256([]byte(content))
		checksums[entry.Name] = hex.EncodeToString(sum[:])
	}

	return checksums, nil
}

func sameChecksums(stored map[string]string, current map[string]string) bool {
	if len(stored) != len(current) {
		return false
	}

	for fileName, storedChecksum := range stored {
		currentChecksum, exists := current[fileName]
		if !exists {
			return false
		}

		if storedChecksum != currentChecksum {
			return false
		}
	}

	return true
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

func filterEntries(entries []domain.DirectoryEntry, projectRoot string, matcher ports.IgnoreMatcher) ([]domain.DirectoryEntry, error) {
	allowedEntries := make([]domain.DirectoryEntry, 0, len(entries))

	for _, entry := range entries {
		if entry.Name == ".git" || entry.Name == ".idx" {
			continue
		}

		relativePath, err := filepath.Rel(projectRoot, entry.Path)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve relative path for %q from %q: got error %v, expected a descendant path", entry.Path, projectRoot, err)
		}

		ignored, err := matcher.Matches(matchPath(relativePath, entry.IsDir))
		if err != nil {
			return nil, err
		}

		if ignored {
			continue
		}

		allowedEntries = append(allowedEntries, entry)
	}

	sort.Slice(allowedEntries, func(left int, right int) bool {
		return allowedEntries[left].Name < allowedEntries[right].Name
	})

	return allowedEntries, nil
}

func matchPath(relativePath string, isDir bool) string {
	normalizedPath := filepath.ToSlash(relativePath)
	if isDir {
		return normalizedPath + "/"
	}

	return normalizedPath
}

func indexFilePath(directoryPath string) string {
	return filepath.Join(directoryPath, ".idx", "index.idx")
}
