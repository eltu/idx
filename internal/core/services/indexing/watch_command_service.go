package indexing

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"

	"idx/internal/core/ports"
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

func (service InitCommandService) resolveWatchContext() (string, ports.IgnoreMatcher, error) {
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

func (service InitCommandService) createFileWatcher(projectRoot string, matcher ports.IgnoreMatcher) (*fsnotify.Watcher, error) {
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

func (service InitCommandService) ensureRootIndex(projectRoot string, matcher ports.IgnoreMatcher) error {
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

func (service InitCommandService) writeWatchHeader(projectRoot string, debounce time.Duration) error {
	projectName := filepath.Base(projectRoot)
	header := fmt.Sprintf("\n%s  %s  %s\n",
		statusWarningStyle.Render("👀  Watching"),
		statusActionStyle.Render(projectName),
		statusMutedStyle.Render("·  Ctrl+C to stop"),
	)
	return service.output.WriteLine(header)
}

func (service InitCommandService) syncAllDirectoriesBeforeWatch(projectRoot string, matcher ports.IgnoreMatcher) error {
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

func (service InitCommandService) addRecursiveWatches(watcher *fsnotify.Watcher, directoryPath string, projectRoot string, matcher ports.IgnoreMatcher) error {
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

func (service InitCommandService) consumeWatchEvents(watcher *fsnotify.Watcher, projectRoot string, matcher ports.IgnoreMatcher, showUpdatedFiles bool, debounce time.Duration) error {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	pendingDirectories := make(map[string]struct{})
	pendingFiles := make(map[string]struct{})
	var timer *time.Timer
	var timerChannel <-chan time.Time

	for {
		select {
		case <-signals:
			msg := fmt.Sprintf("\n%s\n", statusStaleStyle.Render("🛑  Watch stopped."))
			return service.output.WriteLine(msg)
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			return fmt.Errorf("watcher error: %v", err)
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			service.trackEventDirectories(event, projectRoot, matcher, pendingDirectories)
			service.trackEventFiles(event, projectRoot, matcher, pendingFiles)
			if err := service.watchNewDirectory(event, watcher, projectRoot, matcher); err != nil {
				return err
			}

			timer, timerChannel = resetDebounceTimer(timer, debounce)
		case <-timerChannel:
			if err := service.flushWatchedBatch(pendingDirectories, pendingFiles, projectRoot, matcher, showUpdatedFiles); err != nil {
				return err
			}

			pendingDirectories = make(map[string]struct{})
			pendingFiles = make(map[string]struct{})
			timerChannel = nil
		}
	}
}

func resetDebounceTimer(timer *time.Timer, debounce time.Duration) (*time.Timer, <-chan time.Time) {
	if timer == nil {
		timer = time.NewTimer(debounce)
		return timer, timer.C
	}

	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}

	timer.Reset(debounce)
	return timer, timer.C
}

func (service InitCommandService) trackEventDirectories(event fsnotify.Event, projectRoot string, matcher ports.IgnoreMatcher, pending map[string]struct{}) {
	if !shouldTrackFileEvent(event.Op) {
		return
	}

	targetDirectory, ok := eventDirectory(projectRoot, event.Name)
	if !ok || shouldSkipSystemDirectory(targetDirectory) {
		return
	}

	ignored, err := isIgnoredPath(projectRoot, targetDirectory, true, matcher)
	if err != nil || ignored {
		return
	}

	pending[targetDirectory] = struct{}{}
}

func (service InitCommandService) trackEventFiles(event fsnotify.Event, projectRoot string, matcher ports.IgnoreMatcher, pending map[string]struct{}) {
	if !shouldTrackFileEvent(event.Op) {
		return
	}

	cleanedPath := filepath.Clean(event.Name)
	if !isWithinRoot(filepath.Clean(projectRoot), cleanedPath) || hasSystemPathSegment(cleanedPath) {
		return
	}

	if info, err := os.Stat(cleanedPath); err == nil && info.IsDir() {
		return
	}

	ignored, err := isIgnoredPath(projectRoot, cleanedPath, false, matcher)
	if err != nil || ignored {
		return
	}

	relativePath, err := filepath.Rel(projectRoot, cleanedPath)
	if err != nil {
		return
	}

	pending[filepath.ToSlash(relativePath)] = struct{}{}
}

func shouldTrackFileEvent(op fsnotify.Op) bool {
	return op&(fsnotify.Create|fsnotify.Write|fsnotify.Rename|fsnotify.Remove) != 0
}

func hasSystemPathSegment(path string) bool {
	cleaned := filepath.Clean(path)
	parts := strings.Split(filepath.ToSlash(cleaned), "/")
	for _, part := range parts {
		if part == ".git" || part == ".idx" {
			return true
		}
	}

	return false
}

func eventDirectory(projectRoot string, path string) (string, bool) {
	cleanedRoot := filepath.Clean(projectRoot)
	cleanedPath := filepath.Clean(path)

	if !isWithinRoot(cleanedRoot, cleanedPath) {
		return "", false
	}

	info, err := os.Stat(cleanedPath)
	if err == nil && info.IsDir() {
		return cleanedPath, true
	}

	return filepath.Dir(cleanedPath), true
}

func isWithinRoot(projectRoot string, path string) bool {
	relativePath, err := filepath.Rel(projectRoot, path)
	if err != nil {
		return false
	}

	return relativePath == "." || !strings.HasPrefix(relativePath, "..")
}

func shouldSkipSystemDirectory(path string) bool {
	base := filepath.Base(filepath.Clean(path))
	return base == ".git" || base == ".idx"
}

func isIgnoredPath(projectRoot string, path string, isDir bool, matcher ports.IgnoreMatcher) (bool, error) {
	relativePath, err := filepath.Rel(projectRoot, path)
	if err != nil {
		return false, err
	}

	if relativePath == "." {
		return false, nil
	}

	return matcher.Matches(matchPath(relativePath, isDir))
}

func (service InitCommandService) watchNewDirectory(event fsnotify.Event, watcher *fsnotify.Watcher, projectRoot string, matcher ports.IgnoreMatcher) error {
	if event.Op&fsnotify.Create == 0 {
		return nil
	}

	targetDirectory, ok := eventDirectory(projectRoot, event.Name)
	if !ok {
		return nil
	}

	info, err := os.Stat(targetDirectory)
	if err != nil || !info.IsDir() {
		return nil
	}

	return service.addRecursiveWatches(watcher, targetDirectory, projectRoot, matcher)
}

func (service InitCommandService) flushWatchedBatch(pendingDirectories map[string]struct{}, pendingFiles map[string]struct{}, projectRoot string, matcher ports.IgnoreMatcher, showUpdatedFiles bool) error {
	directories := sortedDirectoryBatch(pendingDirectories)
	if len(directories) == 0 {
		return nil
	}

	for _, directoryPath := range directories {
		if err := service.syncDirectoryIndex(directoryPath, projectRoot, matcher); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}

			return err
		}
	}

	return service.writeWatchBatchSummary(directories, pendingFiles)
}

const watchMaxFilesListed = 8

func (service InitCommandService) writeWatchBatchSummary(directories []string, pendingFiles map[string]struct{}) error {
	ts := statusMutedStyle.Render(time.Now().Format("15:04:05"))
	marker := statusSuccessStyle.Render("✦")
	dirLabel := statusMutedStyle.Render(fmt.Sprintf("%d dir(s)", len(directories)))
	files := sortedFileBatch(pendingFiles)
	summary := fmt.Sprintf("  %s  %s  ·  %s  ·  %s", marker, ts, dirLabel, watchFileLabel(files))
	if err := service.output.WriteLine(summary); err != nil {
		return err
	}
	return service.writeWatchFileList(files)
}

func watchFileLabel(files []string) string {
	if len(files) == 0 {
		return statusMutedStyle.Render("structural change")
	}
	return statusMutedStyle.Render(fmt.Sprintf("%d file(s)", len(files)))
}

func (service InitCommandService) writeWatchFileList(files []string) error {
	listed, truncated := truncatedFileList(files)
	for i, f := range listed {
		prefix := watchEntryPrefix(i, len(listed)-1, truncated)
		if err := service.output.WriteLine(fmt.Sprintf("%s %s", prefix, statusPathStyle.Render(f))); err != nil {
			return err
		}
	}
	if truncated > 0 {
		more := statusMutedStyle.Render(fmt.Sprintf("  └─ ... and %d more", truncated))
		if err := service.output.WriteLine(more); err != nil {
			return err
		}
	}
	return service.output.WriteLine("")
}

func truncatedFileList(files []string) ([]string, int) {
	if len(files) <= watchMaxFilesListed {
		return files, 0
	}
	return files[:watchMaxFilesListed], len(files) - watchMaxFilesListed
}

func watchEntryPrefix(i, lastIndex, truncated int) string {
	if i == lastIndex && truncated == 0 {
		return statusMutedStyle.Render("  └─")
	}
	return statusMutedStyle.Render("  ├─")
}

func sortedDirectoryBatch(pending map[string]struct{}) []string {
	directories := make([]string, 0, len(pending))
	for directoryPath := range pending {
		directories = append(directories, directoryPath)
	}

	sort.Strings(directories)
	return directories
}

func (service InitCommandService) writeUpdatedFiles(pendingFiles map[string]struct{}) error {
	files := sortedFileBatch(pendingFiles)
	if len(files) == 0 {
		return service.output.WriteLine("   files: none")
	}

	if err := service.output.WriteLine("   updated files:"); err != nil {
		return err
	}

	for _, filePath := range files {
		if err := service.output.WriteLine("   - " + filePath); err != nil {
			return err
		}
	}

	return nil
}

func sortedFileBatch(pending map[string]struct{}) []string {
	files := make([]string, 0, len(pending))
	for filePath := range pending {
		files = append(files, filePath)
	}

	sort.Strings(files)
	return files
}
