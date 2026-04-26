package search_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"idx/internal/adapters/repository"
	"idx/internal/core/domain"
	"idx/internal/core/ports"
	"idx/internal/core/services/indexing"
	search "idx/internal/core/services/search"
)

type concurrentProjectTree struct {
	root string
}

func (tree concurrentProjectTree) CurrentDir() (string, error) {
	return tree.root, nil
}

func (tree concurrentProjectTree) FindGitRoot(startDir string) (string, error) {
	if strings.HasPrefix(filepath.Clean(startDir), filepath.Clean(tree.root)) {
		return tree.root, nil
	}

	return "", fmt.Errorf("expected path under %q, got %q", tree.root, startDir)
}

func (tree concurrentProjectTree) ReadDir(path string) ([]domain.DirectoryEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	result := make([]domain.DirectoryEntry, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}

		result = append(result, domain.DirectoryEntry{
			Name:            entry.Name(),
			Path:            filepath.Join(path, entry.Name()),
			IsDir:           entry.IsDir(),
			Size:            info.Size(),
			ModTimeUnixNano: info.ModTime().UnixNano(),
		})
	}

	return result, nil
}

func (tree concurrentProjectTree) Exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}

	return false, err
}

func (tree concurrentProjectTree) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

func (tree concurrentProjectTree) WriteFile(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return err
	}

	return os.WriteFile(path, content, 0600)
}

type allowAllIgnoreMatcherFactory struct{}

type allowAllIgnoreMatcher struct{}

func (factory allowAllIgnoreMatcherFactory) New(projectRoot string) (ports.IgnoreMatcher, error) {
	return allowAllIgnoreMatcher{}, nil
}

func (matcher allowAllIgnoreMatcher) Matches(path string) (bool, error) {
	return false, nil
}

type concurrentDiscardOutput struct{}

func (output concurrentDiscardOutput) WriteLine(text string) error {
	return nil
}

func TestSyncAndSearchRunConcurrentlyWithoutInterference(t *testing.T) {
	if !concurrencyTestsEnabled() {
		t.Skip("skipping concurrency tests; set IDX_RUN_CONCURRENCY_TESTS=1 to enable")
	}

	if testing.Short() {
		t.Skip("skipping concurrency stress test in short mode")
	}

	settings := concurrencyTestSettingsFromEnv()
	rootDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(rootDir, ".git"), 0750); err != nil {
		t.Fatalf("expected git marker directory creation to succeed, got %v", err)
	}

	filePaths := createConcurrencyCorpus(t, rootDir, settings.files)
	projectTree := concurrentProjectTree{root: rootDir}
	fileReader := repository.NewOSFileReader()
	indexRepo := repository.NewBinaryIndexRepository(projectTree)
	checksumRepo := repository.NewDirectoryChecksumRepository()
	indexService := indexing.NewInitCommandService(
		projectTree,
		allowAllIgnoreMatcherFactory{},
		concurrentDiscardOutput{},
		fileReader,
		indexing.NewBM25IndexService(),
		indexRepo,
		checksumRepo,
	)
	searchService := search.NewSearchCommandService(projectTree, concurrentDiscardOutput{}, fileReader, indexRepo)

	if err := indexService.Run(); err != nil {
		t.Fatalf("expected initial index build to succeed, got %v", err)
	}

	runSyncSearchConcurrencyScenario(t, indexService, searchService, filePaths, settings)
}

func TestSyncAndSearchRunConcurrentlyAcrossDirectories(t *testing.T) {
	if !concurrencyTestsEnabled() {
		t.Skip("skipping concurrency tests; set IDX_RUN_CONCURRENCY_TESTS=1 to enable")
	}

	if testing.Short() {
		t.Skip("skipping concurrency stress test in short mode")
	}

	settings := concurrencyTestSettingsFromEnv()
	settings.files = 0
	rootDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(rootDir, ".git"), 0750); err != nil {
		t.Fatalf("expected git marker directory creation to succeed, got %v", err)
	}

	filePaths := createConcurrencyCorpusAcrossDirs(t, rootDir, settings.subdirs, settings.filesPerDir)
	projectTree := concurrentProjectTree{root: rootDir}
	fileReader := repository.NewOSFileReader()
	indexRepo := repository.NewBinaryIndexRepository(projectTree)
	checksumRepo := repository.NewDirectoryChecksumRepository()
	indexService := indexing.NewInitCommandService(
		projectTree,
		allowAllIgnoreMatcherFactory{},
		concurrentDiscardOutput{},
		fileReader,
		indexing.NewBM25IndexService(),
		indexRepo,
		checksumRepo,
	)
	searchService := search.NewSearchCommandService(projectTree, concurrentDiscardOutput{}, fileReader, indexRepo)

	if err := indexService.Run(); err != nil {
		t.Fatalf("expected initial multi-dir index build to succeed, got %v", err)
	}

	runSyncSearchConcurrencyScenario(t, indexService, searchService, filePaths, settings)
}

func runSyncSearchConcurrencyScenario(t *testing.T, indexService indexing.InitCommandService, searchService search.SearchCommandService, filePaths []string, settings concurrencyTestSettings) {
	t.Helper()

	errorCh := make(chan error, settings.syncIterations+(settings.searchWorkers*settings.searchIterations))
	var workers sync.WaitGroup
	workers.Add(1 + settings.searchWorkers)

	go func() {
		defer workers.Done()
		for iteration := 0; iteration < settings.syncIterations; iteration++ {
			target := filePaths[iteration%len(filePaths)]
			content := fmt.Sprintf("needle stable token\neditor autosave iteration %d\n", iteration)
			if err := os.WriteFile(target, []byte(content), 0600); err != nil {
				errorCh <- fmt.Errorf("failed to simulate editor save at iteration %d: %w", iteration, err)
				return
			}

			if err := indexService.Sync(); err != nil {
				errorCh <- fmt.Errorf("sync failed at iteration %d: %w", iteration, err)
				return
			}
		}
	}()

	for worker := 0; worker < settings.searchWorkers; worker++ {
		go func(workerID int) {
			defer workers.Done()
			for iteration := 0; iteration < settings.searchIterations; iteration++ {
				err := searchService.RunWithOptions("needle", ports.SearchOptions{FilesOnly: true, Size: 25})
				if err != nil {
					errorCh <- fmt.Errorf("search worker %d failed at iteration %d: %w", workerID, iteration, err)
					return
				}
			}
		}(worker)
	}

	done := make(chan struct{})
	go func() {
		workers.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(settings.timeout):
		t.Fatal("concurrent sync/search test timed out")
	}

	close(errorCh)
	for err := range errorCh {
		t.Fatalf("expected sync and search to coexist without interference, got %v", err)
	}
}

type concurrencyTestSettings struct {
	files            int
	subdirs          int
	filesPerDir      int
	syncIterations   int
	searchWorkers    int
	searchIterations int
	timeout          time.Duration
}

func concurrencyTestSettingsFromEnv() concurrencyTestSettings {
	defaults := defaultConcurrencyTestSettings()
	timeoutSeconds := envInt("IDX_CONCURRENCY_TIMEOUT_SECONDS", int(defaults.timeout.Seconds()))
	if timeoutSeconds < 1 {
		timeoutSeconds = 1
	}

	return concurrencyTestSettings{
		files:            envInt("IDX_CONCURRENCY_FILES", defaults.files),
		subdirs:          envInt("IDX_CONCURRENCY_SUBDIRS", defaults.subdirs),
		filesPerDir:      envInt("IDX_CONCURRENCY_FILES_PER_DIR", defaults.filesPerDir),
		syncIterations:   envInt("IDX_CONCURRENCY_SYNC_ITERATIONS", defaults.syncIterations),
		searchWorkers:    envInt("IDX_CONCURRENCY_SEARCH_WORKERS", defaults.searchWorkers),
		searchIterations: envInt("IDX_CONCURRENCY_SEARCH_ITERATIONS", defaults.searchIterations),
		timeout:          time.Duration(timeoutSeconds) * time.Second,
	}
}

func envInt(name string, defaultValue int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return defaultValue
	}

	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return defaultValue
	}

	return parsed
}

func concurrencyTestsEnabled() bool {
	raw := strings.TrimSpace(os.Getenv("IDX_RUN_CONCURRENCY_TESTS"))
	return raw == "1" || strings.EqualFold(raw, "true")
}

func createConcurrencyCorpus(t *testing.T, rootDir string, files int) []string {
	t.Helper()

	paths := make([]string, 0, files)
	for index := 0; index < files; index++ {
		name := fmt.Sprintf("doc-%03d.txt", index)
		path := filepath.Join(rootDir, name)
		content := fmt.Sprintf("needle baseline token %d\n", index)
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatalf("expected corpus file creation to succeed, got %v", err)
		}
		paths = append(paths, path)
	}

	return paths
}

func createConcurrencyCorpusAcrossDirs(t *testing.T, rootDir string, subdirs int, filesPerDir int) []string {
	t.Helper()

	paths := make([]string, 0, subdirs*filesPerDir)
	anchorPath := filepath.Join(rootDir, "root-anchor.txt")
	if err := os.WriteFile(anchorPath, []byte("needle root anchor\n"), 0600); err != nil {
		t.Fatalf("expected root anchor file creation to succeed, got %v", err)
	}
	paths = append(paths, anchorPath)

	for dirIndex := 0; dirIndex < subdirs; dirIndex++ {
		dirPath := filepath.Join(rootDir, fmt.Sprintf("pkg-%02d", dirIndex))
		if err := os.MkdirAll(dirPath, 0750); err != nil {
			t.Fatalf("expected concurrency subdir creation to succeed, got %v", err)
		}

		for fileIndex := 0; fileIndex < filesPerDir; fileIndex++ {
			name := fmt.Sprintf("doc-%02d-%03d.txt", dirIndex, fileIndex)
			path := filepath.Join(dirPath, name)
			content := fmt.Sprintf("needle baseline token dir=%d file=%d\n", dirIndex, fileIndex)
			if err := os.WriteFile(path, []byte(content), 0600); err != nil {
				t.Fatalf("expected multi-dir corpus file creation to succeed, got %v", err)
			}

			paths = append(paths, path)
		}
	}

	return paths
}
