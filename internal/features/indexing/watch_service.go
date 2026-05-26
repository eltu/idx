package indexing

import (
	"context"
	"fmt"
	"idx/internal/shared/filesystem"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

const defaultWatchDebounceInterval = 750 * time.Millisecond

func (service InitCommandService) watchLoop(showUpdatedFiles bool, debounce time.Duration) error {
	if debounce <= 0 {
		debounce = defaultWatchDebounceInterval
	}
	projectRoot, matcher, err := service.resolveWatchContext()
	if err != nil {
		return err
	}
	watcher, err := service.createFileWatcher(projectRoot, matcher)
	if err != nil {
		return err
	}
	defer func() { _ = watcher.Close() }()
	if err := service.writeWatchHeader(projectRoot, debounce); err != nil {
		return err
	}
	return service.consumeWatchEvents(watcher, projectRoot, matcher, showUpdatedFiles, debounce)
}

func (service InitCommandService) resolveWatchContext() (string, filesystem.IgnoreMatcher, error) {
	currentDir, err := service.projectTree.CurrentDir()
	if err != nil {
		return "", nil, fmt.Errorf("failed to resolve current directory: got error %v, expected a readable working directory", err)
	}
	projectRoot, err := service.projectTree.FindGitRoot(currentDir)
	if err != nil {
		return "", nil, err
	}
	matcher, err := service.matcherFactory.New(projectRoot)
	if err != nil {
		return "", nil, fmt.Errorf("failed to load ignore rules for %q: got error %v, expected a readable .gitignore configuration", projectRoot, err)
	}
	if err := service.ensureRootIndex(projectRoot, matcher); err != nil {
		return "", nil, err
	}
	if err := service.syncAllDirectoriesBeforeWatch(projectRoot, matcher); err != nil {
		return "", nil, err
	}
	return projectRoot, matcher, nil
}

func (service InitCommandService) createFileWatcher(projectRoot string, matcher filesystem.IgnoreMatcher) (*fsnotify.Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to start file watcher for %q: got error %v, expected watcher initialization", projectRoot, err)
	}
	if err := service.addRecursiveWatches(watcher, projectRoot, projectRoot, matcher); err != nil {
		_ = watcher.Close()
		return nil, err
	}
	return watcher, nil
}

func (service InitCommandService) ensureRootIndex(projectRoot string, matcher filesystem.IgnoreMatcher) error {
	rootIndexPath := indexFilePath(projectRoot)
	hasRootIndex, err := service.projectTree.Exists(rootIndexPath)
	if err != nil {
		return err
	}

	if hasRootIndex {
		return nil
	}

	creating := fmt.Sprintf("  %s  %s",
		statusWarningStyle.Render("ℹ"),
		statusMutedStyle.Render("No index found. Creating initial index..."),
	)
	if err := service.output.WriteLine(creating); err != nil {
		return err
	}

	allDirs, err := service.collectAllDirectories(projectRoot, projectRoot, matcher)
	if err != nil {
		return err
	}
	if err := service.indexAllParallel(context.Background(), allDirs, projectRoot, matcher); err != nil {
		return err
	}

	created := fmt.Sprintf("  %s  %s",
		statusSuccessStyle.Render("✓"),
		statusMutedStyle.Render("Initial index created."),
	)
	return service.output.WriteLine(created)
}

func (service InitCommandService) writeWatchHeader(projectRoot string, _ time.Duration) error {
	projectName := filepath.Base(projectRoot)
	header := fmt.Sprintf("\n%s  %s  %s\n",
		statusWarningStyle.Render("👀  Watching"),
		statusActionStyle.Render(projectName),
		statusMutedStyle.Render("·  Ctrl+C to stop"),
	)
	return service.output.WriteLine(header)
}

func (service InitCommandService) syncAllDirectoriesBeforeWatch(projectRoot string, matcher filesystem.IgnoreMatcher) error {
	indexedDirectories, err := IndexedDirectories(service.projectTree, projectRoot)
	if err != nil {
		return err
	}

	eligible, err := eligibleDirectories(service.projectTree, projectRoot, matcher)
	if err != nil {
		return err
	}

	stale := staleIndexedDirectories(indexedDirectories, eligible)
	if err := service.removeStaleDirectories(stale); err != nil {
		return err
	}

	return service.syncEligibleDirectories(eligible, projectRoot, matcher)
}

func (service InitCommandService) addRecursiveWatches(watcher *fsnotify.Watcher, directoryPath, projectRoot string, matcher filesystem.IgnoreMatcher) error {
	if shouldSkipSystemDirectory(directoryPath) {
		return nil
	}

	ignored, err := isIgnoredPath(projectRoot, directoryPath, true, matcher)
	if err != nil {
		return err
	}
	if ignored {
		return nil
	}

	if err := addWatchPath(watcher, directoryPath); err != nil {
		return err
	}

	entries, err := service.projectTree.ReadDir(directoryPath)
	if err != nil {
		return fmt.Errorf("failed to read directory %q: got error %v, expected a readable directory", directoryPath, err)
	}

	for _, entry := range entries {
		if !entry.IsDir {
			continue
		}

		if err := service.addRecursiveWatches(watcher, entry.Path, projectRoot, matcher); err != nil {
			return err
		}
	}

	return nil
}

func addWatchPath(watcher *fsnotify.Watcher, directoryPath string) error {
	err := watcher.Add(directoryPath)
	if err == nil {
		return nil
	}

	if strings.Contains(err.Error(), "exists") {
		return nil
	}

	return fmt.Errorf("failed to watch directory %q: got error %v, expected readable path", directoryPath, err)
}
