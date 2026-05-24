package indexing

import (
	"idx/internal/shared/filesystem"
	"fmt"
	"path/filepath"
	"runtime"
	"sync"

)

const errReadDirFmt = "failed to read directory %q: got error %v, expected a readable directory"

// IndexedDirectories walks the project tree from projectRoot and returns all directories
// that contain an .idx/index.idx file.
func IndexedDirectories(projectTree filesystem.ProjectTree, projectRoot string) ([]string, error) {
	directories := make([]string, 0)
	if err := collectIndexedDirectories(projectTree, projectRoot, &directories); err != nil {
		return nil, err
	}

	return directories, nil
}

func eligibleDirectories(projectTree filesystem.ProjectTree, projectRoot string, matcher filesystem.IgnoreMatcher) ([]string, error) {
	directories := make([]string, 0)
	if err := collectEligibleDirectories(projectTree, projectRoot, projectRoot, matcher, &directories); err != nil {
		return nil, err
	}

	return directories, nil
}

func collectIndexedDirectories(projectTree filesystem.ProjectTree, directoryPath string, directories *[]string) error {
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
		return fmt.Errorf(errReadDirFmt, directoryPath, err)
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

func collectEligibleDirectories(projectTree filesystem.ProjectTree, directoryPath string, projectRoot string, matcher filesystem.IgnoreMatcher, directories *[]string) error {
	*directories = append(*directories, directoryPath)

	entries, err := projectTree.ReadDir(directoryPath)
	if err != nil {
		return fmt.Errorf(errReadDirFmt, directoryPath, err)
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

func directoryEntries(entries []filesystem.DirectoryEntry) []filesystem.DirectoryEntry {
	directories := make([]filesystem.DirectoryEntry, 0)
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
// indexedDirWalker holds shared state for the parallel indexed-directory walk.
type indexedDirWalker struct {
	projectTree filesystem.ProjectTree
	sem         chan struct{}
	errCh       chan error
	mu          sync.Mutex
	result      []string
	wg          sync.WaitGroup
}

func parallelIndexedDirectories(projectTree filesystem.ProjectTree, root string) ([]string, error) {
	w := &indexedDirWalker{
		projectTree: projectTree,
		sem:         make(chan struct{}, runtime.NumCPU()),
		errCh:       make(chan error, 1),
	}
	w.wg.Add(1)
	go w.walk(root)
	w.wg.Wait()
	close(w.errCh)
	if err := <-w.errCh; err != nil {
		return nil, err
	}
	return w.result, nil
}

func (w *indexedDirWalker) scanDirectory(path string) (hasIndex bool, entries []filesystem.DirectoryEntry, err error) {
	w.sem <- struct{}{}
	defer func() { <-w.sem }()
	hasIndex, err = w.projectTree.Exists(filepath.Join(path, ".idx", "index.idx"))
	if err != nil {
		return false, nil, err
	}
	entries, err = w.projectTree.ReadDir(path)
	return hasIndex, entries, err
}

func (w *indexedDirWalker) walk(path string) {
	defer w.wg.Done()
	hasIndex, entries, err := w.scanDirectory(path)
	if err != nil {
		select {
		case w.errCh <- fmt.Errorf(errReadDirFmt, path, err):
		default:
		}
		return
	}
	if hasIndex {
		w.mu.Lock()
		w.result = append(w.result, path)
		w.mu.Unlock()
	}
	for _, entry := range entries {
		if !entry.IsDir || entry.Name == ".git" || entry.Name == ".idx" {
			continue
		}
		w.wg.Add(1)
		go w.walk(entry.Path)
	}
}

// eligibleDirectory pairs a directory path with its pre-computed indexable file entries.
// Carrying the entries avoids repeated ReadDir calls in downstream status stages.
type eligibleDirectory struct {
	path        string
	fileEntries []filesystem.DirectoryEntry
}

// parallelEligibleDirectories is a concurrent variant of eligibleDirectories.
// It returns each eligible directory together with its indexable file entries so
// callers can skip redundant ReadDir calls.
func parallelEligibleDirectories(projectTree filesystem.ProjectTree, root, projectRoot string, matcher filesystem.IgnoreMatcher) ([]eligibleDirectory, error) {
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
			case errCh <- fmt.Errorf(errReadDirFmt, path, readErr):
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
