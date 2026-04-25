package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"sort"

	"idx/internal/core/domain"
	"idx/internal/core/ports"
)

type InitCommandService struct {
	projectTree    ports.ProjectTree
	matcherFactory ports.IgnoreMatcherFactory
	output         ports.TextOutput
	fileReader     ports.FileReader
	indexer        ports.BM25Indexer
	indexRepo      IndexRepository
	checksumRepo   DirectoryChecksumRepository
}

// IndexRepository defines saving index to storage.
type IndexRepository interface {
	SaveIndex(directoryPath string, index *domain.InvertedIndex) error
	LoadIndex(directoryPath string) (*domain.InvertedIndex, error)
}

// DirectoryChecksumRepository defines checksum metadata storage per directory.
type DirectoryChecksumRepository interface {
	Load(directoryPath string) (map[string]string, bool, error)
	Save(directoryPath string, checksums map[string]string) error
}

// NewInitCommandService builds the init use case.
// Example: service := NewInitCommandService(projectTree, matcherFactory, output, fileReader, indexer, indexRepo).
func NewInitCommandService(projectTree ports.ProjectTree, matcherFactory ports.IgnoreMatcherFactory, output ports.TextOutput, fileReader ports.FileReader, indexer ports.BM25Indexer, indexRepo IndexRepository, checksumRepo DirectoryChecksumRepository) InitCommandService {
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
	return service.runIndex()
}

func (service InitCommandService) Sync() error {
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

	directories, err := indexedDirectories(service.projectTree, projectRoot)
	if err != nil {
		return err
	}

	eligibleDirectories, err := eligibleDirectories(service.projectTree, projectRoot, matcher)
	if err != nil {
		return err
	}

	staleDirectories := staleIndexedDirectories(directories, eligibleDirectories)
	for _, directoryPath := range staleDirectories {
		if err := service.removeDirectoryIndex(directoryPath); err != nil {
			return err
		}
	}

	for _, directoryPath := range eligibleDirectories {
		if err := service.syncDirectoryIndex(directoryPath, projectRoot, matcher); err != nil {
			return err
		}
	}

	return service.output.WriteLine("✅ Project indices synchronized.")
}

func (service InitCommandService) removeDirectoryIndex(directoryPath string) error {
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
		return service.output.WriteLine("ℹ️ Este projeto ja possui indice. Voce pode executar idx search.")
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

	checksums, err := service.directoryChecksums(fileEntries)
	if err != nil {
		return err
	}

	shouldReindex, err := service.shouldReindexDirectory(directoryPath, checksums)
	if err != nil {
		return err
	}

	if !shouldReindex {
		return nil
	}

	if len(fileEntries) == 0 {
		emptyIndex := domain.NewInvertedIndex()
		if err := service.indexRepo.SaveIndex(directoryPath, emptyIndex); err != nil {
			return err
		}

		return service.checksumRepo.Save(directoryPath, checksums)
	}

	if err := service.buildAndSaveIndex(directoryPath, fileEntries); err != nil {
		return err
	}

	return service.checksumRepo.Save(directoryPath, checksums)
}

func (service InitCommandService) shouldReindexDirectory(directoryPath string, currentChecksums map[string]string) (bool, error) {
	currentIndexPath := indexFilePath(directoryPath)
	hasIndex, err := service.projectTree.Exists(currentIndexPath)
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

func (service InitCommandService) buildAndSaveIndex(directoryPath string, fileEntries []domain.DirectoryEntry) error {
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

	return nil
}

func (service InitCommandService) indexChildren(entries []domain.DirectoryEntry, projectRoot string, matcher ports.IgnoreMatcher) error {
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
