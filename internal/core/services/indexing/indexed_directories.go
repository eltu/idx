package indexing

import (
	"fmt"
	"path/filepath"
	"runtime"
	"sync"

	"idx/internal/core/domain"
	"idx/internal/core/ports"
)

// IndexedDirectories walks the project tree from projectRoot and returns all directories
// that contain an .idx/index.idx file.
func IndexedDirectories(projectTree ports.ProjectTree, projectRoot string) ([]string, error) {
	directories := make([]string, 0)
	if err := collectIndexedDirectories(projectTree, projectRoot, &directories); err != nil {
		return nil, err
	}

	return directories, nil
}

func eligibleDirectories(projectTree ports.ProjectTree, projectRoot string, matcher ports.IgnoreMatcher) ([]string, error) {
	directories := make([]string, 0)
	if err := collectEligibleDirectories(projectTree, projectRoot, projectRoot, matcher, &directories); err != nil {
		return nil, err
	}

	return directories, nil
}

func collectIndexedDirectories(projectTree ports.ProjectTree, directoryPath string, directories *[]string) error {
	indexPath := filepath.Join(directoryPath, ".idx", "index.idx")
	hasIndex, err := projectTree.Exists(indexPath)
	if err != nil {
		return err
	}

	if hasIndex {
		*directories = append(*directories, directoryPath)
	}

	entries, err := projectTree.ReadDir(directoryPath)
	if err != nil {
		return fmt.Errorf("failed to read directory %q: got error %v, expected a readable directory", directoryPath, err)
	}

	for _, entry := range entries {
		if !entry.IsDir || entry.Name == ".git" || entry.Name == ".idx" {
			continue
		}

		if err := collectIndexedDirectories(projectTree, entry.Path, directories); err != nil {
			return err
		}
	}

	return nil
}

func collectEligibleDirectories(projectTree ports.ProjectTree, directoryPath string, projectRoot string, matcher ports.IgnoreMatcher, directories *[]string) error {
	*directories = append(*directories, directoryPath)

	entries, err := projectTree.ReadDir(directoryPath)
	if err != nil {
		return fmt.Errorf("failed to read directory %q: got error %v, expected a readable directory", directoryPath, err)
	}

	allowedEntries, err := filterEntries(entries, projectRoot, matcher)
	if err != nil {
		return err
	}

	directoriesToVisit := directoryEntries(allowedEntries)
	for _, entry := range directoriesToVisit {
		if err := collectEligibleDirectories(projectTree, entry.Path, projectRoot, matcher, directories); err != nil {
			return err
		}
	}

	return nil
}

func directoryEntries(entries []domain.DirectoryEntry) []domain.DirectoryEntry {
	directories := make([]domain.DirectoryEntry, 0)
	for _, entry := range entries {
		if !entry.IsDir {
			continue
		}

		directories = append(directories, entry)
	}

	return directories
}

// parallelIndexedDirectories is a concurrent variant of IndexedDirectories.
// It fans out one goroutine per subdirectory, bounded by a semaphore of size runtime.NumCPU(),
// so only that many ReadDir/Exists calls are in flight at once.
func parallelIndexedDirectories(projectTree ports.ProjectTree, root string) ([]string, error) {
	sem := make(chan struct{}, runtime.NumCPU())
	errCh := make(chan error, 1)

	var mu sync.Mutex
	var result []string
	var wg sync.WaitGroup

	var walk func(path string)
	walk = func(path string) {
		defer wg.Done()

		sem <- struct{}{}
		indexPath := filepath.Join(path, ".idx", "index.idx")
		hasIndex, existsErr := projectTree.Exists(indexPath)
		entries, readErr := projectTree.ReadDir(path)
		<-sem

		if existsErr != nil {
			select {
			case errCh <- existsErr:
			default:
			}
			return
		}
		if readErr != nil {
			select {
			case errCh <- fmt.Errorf("failed to read directory %q: got error %v, expected a readable directory", path, readErr):
			default:
			}
			return
		}

		if hasIndex {
			mu.Lock()
			result = append(result, path)
			mu.Unlock()
		}

		for _, entry := range entries {
			if !entry.IsDir || entry.Name == ".git" || entry.Name == ".idx" {
				continue
			}
			wg.Add(1)
			go walk(entry.Path)
		}
	}

	wg.Add(1)
	go walk(root)
	wg.Wait()
	close(errCh)

	if err := <-errCh; err != nil {
		return nil, err
	}
	return result, nil
}

// eligibleDirectory pairs a directory path with its pre-computed indexable file entries.
// Carrying the entries avoids repeated ReadDir calls in downstream status stages.
type eligibleDirectory struct {
	path        string
	fileEntries []domain.DirectoryEntry
}

// parallelEligibleDirectories is a concurrent variant of eligibleDirectories.
// It returns each eligible directory together with its indexable file entries so
// callers can skip redundant ReadDir calls.
func parallelEligibleDirectories(projectTree ports.ProjectTree, root, projectRoot string, matcher ports.IgnoreMatcher) ([]eligibleDirectory, error) {
	sem := make(chan struct{}, runtime.NumCPU())
	errCh := make(chan error, 1)

	var mu sync.Mutex
	var result []eligibleDirectory
	var wg sync.WaitGroup

	var walk func(path string)
	walk = func(path string) {
		defer wg.Done()

		sem <- struct{}{}
		entries, readErr := projectTree.ReadDir(path)
		<-sem

		if readErr != nil {
			select {
			case errCh <- fmt.Errorf("failed to read directory %q: got error %v, expected a readable directory", path, readErr):
			default:
			}
			return
		}

		allowedEntries, filterErr := filterEntries(entries, projectRoot, matcher)
		if filterErr != nil {
			select {
			case errCh <- filterErr:
			default:
			}
			return
		}

		mu.Lock()
		result = append(result, eligibleDirectory{path: path, fileEntries: filesFromEntries(allowedEntries)})
		mu.Unlock()

		for _, entry := range directoryEntries(allowedEntries) {
			wg.Add(1)
			go walk(entry.Path)
		}
	}

	wg.Add(1)
	go walk(root)
	wg.Wait()
	close(errCh)

	if err := <-errCh; err != nil {
		return nil, err
	}
	return result, nil
}
