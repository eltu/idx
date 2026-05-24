package indexing

import (
	"idx/internal/shared/filesystem"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"time"

	"golang.org/x/sync/errgroup"

)

func (service InitCommandService) runIndex() error {
	currentDir, projectRoot, err := service.resolveIndexPaths()
	if err != nil {
		return err
	}
	if err := service.ensureIdxRuleInGitIgnore(projectRoot); err != nil {
		return err
	}
	hasIndex, err := service.projectTree.Exists(indexFilePath(currentDir))
	if err != nil {
		return err
	}
	if hasIndex {
		return service.output.WriteLine("ℹ️ This project is already indexed. You can run idx search.")
	}
	return service.initIndexing(currentDir, projectRoot)
}

func (service InitCommandService) resolveIndexPaths() (string, string, error) {
	currentDir, err := service.projectTree.CurrentDir()
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve current directory: got error %v, expected a readable working directory", err)
	}
	projectRoot, err := service.projectTree.FindGitRoot(currentDir)
	if err != nil {
		return "", "", err
	}
	return currentDir, projectRoot, nil
}

func (service InitCommandService) initIndexing(currentDir, projectRoot string) error {
	matcher, err := service.matcherFactory.New(projectRoot)
	if err != nil {
		return fmt.Errorf("failed to load ignore rules for %q: got error %v, expected a readable .gitignore configuration", projectRoot, err)
	}
	service.initProgress.StartCounting()
	allDirs, err := service.collectAllDirectories(currentDir, projectRoot, matcher)
	if err != nil {
		service.initProgress.Finish()
		return err
	}
	service.initProgress.SetTotal(len(allDirs))
	err = service.indexAllParallel(service.initProgress.Context(), allDirs, projectRoot, matcher)
	service.initProgress.Finish()
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

// collectAllDirectories returns all eligible directory paths under dirPath via DFS.
func (service InitCommandService) collectAllDirectories(dirPath, projectRoot string, matcher filesystem.IgnoreMatcher) ([]string, error) {
	entries, err := service.projectTree.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %q: got error %v, expected a readable directory", dirPath, err)
	}
	allowedEntries, err := filterEntries(entries, projectRoot, matcher)
	if err != nil {
		return nil, err
	}
	dirs := []string{dirPath}
	for _, entry := range allowedEntries {
		if !entry.IsDir {
			continue
		}
		children, err := service.collectAllDirectories(entry.Path, projectRoot, matcher)
		if err != nil {
			return nil, err
		}
		dirs = append(dirs, children...)
	}
	return dirs, nil
}

// indexAllParallel indexes dirs concurrently with a worker limit of runtime.NumCPU().
// Each directory writes to its own isolated path so no coordination between workers is needed.
func (service InitCommandService) indexAllParallel(ctx context.Context, dirs []string, projectRoot string, matcher filesystem.IgnoreMatcher) error {
	g, gCtx := errgroup.WithContext(ctx)
	g.SetLimit(runtime.NumCPU())
	for _, dir := range dirs {
		dir := dir
		g.Go(func() error {
			if gCtx.Err() != nil {
				return context.Canceled
			}
			if err := service.syncDirectoryIndex(dir, projectRoot, matcher); err != nil {
				return err
			}
			service.initProgress.IncrementDir(dir)
			return nil
		})
	}
	return g.Wait()
}

func (service InitCommandService) syncDirectoryIndex(directoryPath string, projectRoot string, matcher filesystem.IgnoreMatcher) error {
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

func (service InitCommandService) indexableFileEntries(directoryPath string, projectRoot string, matcher filesystem.IgnoreMatcher) ([]filesystem.DirectoryEntry, error) {
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

func (service InitCommandService) reindexState(directoryPath string, fileEntries []filesystem.DirectoryEntry) (DirectoryChecksumSnapshot, map[string]struct{}, bool, error) {
	hasIndex, err := service.hasDirectoryIndex(directoryPath)
	if err != nil {
		return DirectoryChecksumSnapshot{}, nil, false, err
	}

	storedSnapshot, hasSnapshot, err := service.loadChecksumSnapshot(directoryPath)
	if err != nil {
		return DirectoryChecksumSnapshot{}, nil, false, err
	}

	currentSnapshot, changed, changedFileNames, err := service.computeDirectorySnapshot(fileEntries, storedSnapshot)
	if err != nil {
		return DirectoryChecksumSnapshot{}, nil, false, err
	}

	shouldReindex := !hasIndex || !hasSnapshot || changed
	return currentSnapshot, changedFileNames, shouldReindex, nil
}

func (service InitCommandService) buildAndSaveIndex(directoryPath string, fileEntries []filesystem.DirectoryEntry, snapshot DirectoryChecksumSnapshot, changedFileNames map[string]struct{}) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}
	documents, err := service.buildIndexDocuments(fileEntries)
	if err != nil {
		return err
	}
	index, err := service.indexer.BuildIndex(documents)
	if err != nil {
		return fmt.Errorf("failed to build BM25 index for %q: got error %v, expected valid document content", directoryPath, err)
	}
	if err := service.indexRepo.SaveIndex(directoryPath, index); err != nil {
		return err
	}
	logEntries := buildChangedLogEntries(fileEntries, snapshot, changedFileNames, time.Now().UTC())
	return appendIndexedFilesLog(directoryPath, logEntries)
}

func (service InitCommandService) buildIndexDocuments(fileEntries []filesystem.DirectoryEntry) ([]IndexDocument, error) {
	documents := make([]IndexDocument, 0, len(fileEntries))
	for _, entry := range fileEntries {
		content, err := service.fileReader.ReadFile(entry.Path)
		if err != nil {
			return nil, err
		}
		documents = append(documents, IndexDocument{
			Name:    entry.Name,
			Path:    entry.Path,
			Content: content,
		})
	}
	return documents, nil
}

func buildChangedLogEntries(fileEntries []filesystem.DirectoryEntry, snapshot DirectoryChecksumSnapshot, changedFileNames map[string]struct{}, indexedAt time.Time) []indexedFileLogEntry {
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

func indexFilePath(directoryPath string) string {
	return filepath.Join(directoryPath, ".idx", "index.idx")
}
