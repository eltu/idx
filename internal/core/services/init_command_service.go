package services

import (
	"encoding/json"
	"fmt"
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
}

// IndexRepository defines saving index to storage.
type IndexRepository interface {
	SaveIndex(directoryPath string, index *domain.InvertedIndex) error
	LoadIndex(directoryPath string) (*domain.InvertedIndex, error)
}

// NewInitCommandService builds the init use case.
// Example: service := NewInitCommandService(projectTree, matcherFactory, output, fileReader, indexer, indexRepo).
func NewInitCommandService(projectTree ports.ProjectTree, matcherFactory ports.IgnoreMatcherFactory, output ports.TextOutput, fileReader ports.FileReader, indexer ports.BM25Indexer, indexRepo IndexRepository) InitCommandService {
	return InitCommandService{
		projectTree:    projectTree,
		matcherFactory: matcherFactory,
		output:         output,
		fileReader:     fileReader,
		indexer:        indexer,
		indexRepo:      indexRepo,
	}
}

func (service InitCommandService) Run() error {
	return service.RunWithDebug(false)
}

func (service InitCommandService) RunWithDebug(debug bool) error {
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
		if err := service.output.WriteLine("ℹ️ Este projeto ja possui indice. Voce pode executar idx search."); err != nil {
			return err
		}

		if !debug {
			return nil
		}

		return service.writeDebugIndex(currentDir)
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

	if err := service.output.WriteLine("✅ Index created. You can now run idx search."); err != nil {
		return err
	}

	if !debug {
		return nil
	}

	return service.writeDebugIndex(currentDir)
}

func (service InitCommandService) writeDebugIndex(directoryPath string) error {
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
	entries, err := service.projectTree.ReadDir(directoryPath)
	if err != nil {
		return fmt.Errorf("failed to read directory %q: got error %v, expected a readable directory", directoryPath, err)
	}

	allowedEntries, err := filterEntries(entries, projectRoot, matcher)
	if err != nil {
		return err
	}

	// Separate files and directories
	fileEntries := make([]domain.DirectoryEntry, 0)
	dirEntries := make([]domain.DirectoryEntry, 0)
	for _, entry := range allowedEntries {
		if entry.IsDir {
			dirEntries = append(dirEntries, entry)
		} else {
			fileEntries = append(fileEntries, entry)
		}
	}

	// Build BM25 index from files in this directory only
	if len(fileEntries) > 0 {
		if err := service.buildAndSaveIndex(directoryPath, fileEntries); err != nil {
			return err
		}
	} else {
		// Even empty directories get an empty index
		emptyIndex := domain.NewInvertedIndex()
		if err := service.indexRepo.SaveIndex(directoryPath, emptyIndex); err != nil {
			return err
		}
	}

	// Recursively index subdirectories
	return service.indexChildren(dirEntries, projectRoot, matcher)
}

func (service InitCommandService) buildAndSaveIndex(directoryPath string, fileEntries []domain.DirectoryEntry) error {
	// Read all files and build documents map
	documents := make(map[string]string)
	for _, entry := range fileEntries {
		content, err := service.fileReader.ReadFile(entry.Path)
		if err != nil {
			return err
		}
		documents[entry.Name] = content
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
