package indexing

import (
	"fmt"
	"idx/internal/shared/filesystem"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

type statusFileReport struct {
	Name       string
	Updated    bool
	ModifiedAt time.Time
}

type statusDirectoryReport struct {
	Path             string
	Files            []statusFileReport
	ShouldReindex    bool
	StructuralChange bool
}

type statusSummary struct {
	CheckedDirectories int
	UpdatedDirectories int
	CheckedFiles       int
	UpdatedFiles       int
	LatestModifiedAt   time.Time
	HasLatest          bool
}

func (service InitCommandService) Status() error {
	return service.runStatus(false)
}

// StatusWithContext is like Status but embeds config file info in the overview panel.
func (service InitCommandService) StatusWithContext(configFilePath string, configOverrides []string) error {
	service.statusConfigFilePath = configFilePath
	service.statusConfigOverrides = configOverrides
	return service.runStatus(false)
}

func (service InitCommandService) StatusWithProfile(profile bool) error {
	return service.runStatus(profile)
}

type statusGatherResult struct {
	projectRoot      string
	directories      []string
	reports          []statusDirectoryReport
	summary          statusSummary
	staleDirectories []string
}

func (service InitCommandService) runStatus(profile bool) error {
	stop := service.startStatusSpinner()
	result, err := service.gatherStatus()
	stop()
	if err != nil {
		return err
	}
	return service.writeStatusResult(profile, result.projectRoot, result.directories, result.reports, result.summary, result.staleDirectories)
}

func (service InitCommandService) gatherStatus() (statusGatherResult, error) {
	if err := service.validateDependencies(); err != nil {
		return statusGatherResult{}, err
	}
	projectRoot, matcher, err := service.statusMatcher()
	if err != nil {
		return statusGatherResult{}, err
	}
	directories, eligible, err := service.classifyDirectories(projectRoot, matcher)
	if err != nil {
		return statusGatherResult{}, err
	}
	if len(directories) == 0 {
		return statusGatherResult{}, service.writeNoIndexError(projectRoot)
	}
	if err := service.validateIndexCoverage(projectRoot, directories, eligible); err != nil {
		return statusGatherResult{}, err
	}
	entriesByPath := buildEntriesIndex(eligible)
	reports, summary, staleDirectories, err := service.collectStatusReports(directories, projectRoot, matcher, entriesByPath)
	if err != nil {
		return statusGatherResult{}, err
	}
	return statusGatherResult{projectRoot, directories, reports, summary, staleDirectories}, nil
}

var statusSpinnerFrames = []string{`|`, `/`, `-`, `\`}

// spinnerClearWidth must be at least as wide as the spinner message rendered in the terminal.
const spinnerClearWidth = 40

// startStatusSpinner prints an animated spinner while status is being gathered.
// Uses type assertion so outputs that don't support inline writing silently skip the animation.
// The returned stop func blocks until the goroutine has cleared the spinner line.
type statusSpinnerWriter interface{ WriteInline(string) error }

func (service InitCommandService) startStatusSpinner() func() {
	inlineWriter, ok := service.output.(statusSpinnerWriter)
	if !ok {
		return func() { /* no-op: output does not support inline writing */ }
	}
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	ticker := time.NewTicker(100 * time.Millisecond)
	go runStatusSpinnerLoop(inlineWriter, done, ticker, &wg)
	return func() {
		close(done)
		wg.Wait()
	}
}

func runStatusSpinnerLoop(w statusSpinnerWriter, done <-chan struct{}, ticker *time.Ticker, wg *sync.WaitGroup) {
	defer wg.Done()
	defer ticker.Stop()
	started := false
	for i := 0; ; i++ {
		select {
		case <-done:
			if started {
				_ = w.WriteInline("\r" + strings.Repeat(" ", spinnerClearWidth) + "\r")
			}
			return
		case <-ticker.C:
			frame := statusSpinnerStyle.Render(statusSpinnerFrames[i%4])
			label := statusCheckingLabelStyle.Render("Checking index status...")
			prefix := "\r"
			if !started {
				prefix = "\n\r"
				started = true
			}
			_ = w.WriteInline(fmt.Sprintf("%s  %s %s", prefix, label, frame))
		}
	}
}

// classifyDirectories runs parallelIndexedDirectories and parallelEligibleDirectories concurrently.
// Both are independent read-only tree walks, so they can overlap in time.
func (service InitCommandService) classifyDirectories(projectRoot string, matcher filesystem.IgnoreMatcher) (indexed []string, eligible []eligibleDirectory, err error) {
	var indexedErr, eligibleErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		indexed, indexedErr = parallelIndexedDirectories(service.projectTree, projectRoot)
	}()
	go func() {
		defer wg.Done()
		eligible, eligibleErr = parallelEligibleDirectories(service.projectTree, projectRoot, projectRoot, matcher)
	}()
	wg.Wait()
	if indexedErr != nil {
		return nil, nil, indexedErr
	}
	return indexed, eligible, eligibleErr
}

// buildEntriesIndex indexes eligible directories by path for O(1) lookup.
func buildEntriesIndex(eligible []eligibleDirectory) map[string][]filesystem.DirectoryEntry {
	m := make(map[string][]filesystem.DirectoryEntry, len(eligible))
	for _, d := range eligible {
		m[d.path] = d.fileEntries
	}
	return m
}

func (service InitCommandService) validateIndexCoverage(projectRoot string, indexed []string, eligible []eligibleDirectory) error {
	withFiles := make([]string, 0, len(eligible))
	for _, d := range eligible {
		if len(d.fileEntries) > 0 {
			withFiles = append(withFiles, d.path)
		}
	}
	missing := missingIndexDirectories(indexed, withFiles)
	if len(missing) > 0 {
		return service.writeUnindexedDirectoriesError(projectRoot, missing)
	}
	return nil
}

func (service InitCommandService) collectStatusReports(directories []string, projectRoot string, matcher filesystem.IgnoreMatcher, entriesByPath map[string][]filesystem.DirectoryEntry) ([]statusDirectoryReport, statusSummary, []string, error) {
	reports := make([]statusDirectoryReport, len(directories))

	var g errgroup.Group
	g.SetLimit(runtime.NumCPU())
	for i, directoryPath := range directories {
		i, directoryPath := i, directoryPath
		g.Go(func() error {
			report, err := service.collectDirectoryReport(directoryPath, projectRoot, matcher, entriesByPath)
			if err != nil {
				return err
			}
			reports[i] = report
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, statusSummary{}, nil, err
	}

	summary := statusSummary{CheckedDirectories: len(directories)}
	staleDirectories := make([]string, 0)
	for _, report := range reports {
		summary = updateStatusSummary(summary, report)
		if report.ShouldReindex {
			staleDirectories = append(staleDirectories, report.Path)
		} else {
			summary.UpdatedDirectories++
		}
	}

	return reports, summary, staleDirectories, nil
}

func (service InitCommandService) collectDirectoryReport(directoryPath, projectRoot string, matcher filesystem.IgnoreMatcher, entriesByPath map[string][]filesystem.DirectoryEntry) (statusDirectoryReport, error) {
	fileEntries, ok := entriesByPath[directoryPath]
	if !ok {
		// fallback: indexed dir became gitignored since last sync
		var err error
		fileEntries, err = service.indexableFileEntries(directoryPath, projectRoot, matcher)
		if err != nil {
			return statusDirectoryReport{}, err
		}
	}
	return service.verifyDirectoryIndexCurrent(directoryPath, fileEntries)
}

func (service InitCommandService) statusMatcher() (string, filesystem.IgnoreMatcher, error) {
	currentDir, err := service.projectTree.CurrentDir()
	if err != nil {
		return "", nil, fmt.Errorf("failed to resolve current directory: got error %v, expected a readable working directory", err)
	}

	projectRoot, err := service.projectTree.FindGitRoot(currentDir)
	if err != nil {
		return "", nil, service.writeNotGitRepoError(currentDir)
	}

	matcher, err := service.matcherFactory.New(projectRoot)
	if err != nil {
		return "", nil, fmt.Errorf("failed to load ignore rules for %q: got error %v, expected a readable .gitignore configuration", projectRoot, err)
	}

	return projectRoot, matcher, nil
}

func (service InitCommandService) verifyDirectoryIndexCurrent(directoryPath string, fileEntries []filesystem.DirectoryEntry) (statusDirectoryReport, error) {
	_, changedFileNames, shouldReindex, err := service.reindexState(directoryPath, fileEntries)
	if err != nil {
		return statusDirectoryReport{}, err
	}

	files := statusFileReports(fileEntries, changedFileNames)
	structuralChange := shouldReindex && len(changedFileNames) == 0

	return statusDirectoryReport{
		Path:             directoryPath,
		Files:            files,
		ShouldReindex:    shouldReindex,
		StructuralChange: structuralChange,
	}, nil
}

func statusFileReports(fileEntries []filesystem.DirectoryEntry, changedFileNames map[string]struct{}) []statusFileReport {
	reports := make([]statusFileReport, 0, len(fileEntries))
	for _, entry := range fileEntries {
		_, changed := changedFileNames[entry.Name]
		reports = append(reports, statusFileReport{
			Name:       entry.Name,
			Updated:    !changed,
			ModifiedAt: time.Unix(0, entry.ModTimeUnixNano).UTC(),
		})
	}

	return reports
}

func updateStatusSummary(summary statusSummary, report statusDirectoryReport) statusSummary {
	for _, file := range report.Files {
		summary.CheckedFiles++
		if file.Updated {
			summary.UpdatedFiles++
		}

		if !summary.HasLatest || file.ModifiedAt.After(summary.LatestModifiedAt) {
			summary.LatestModifiedAt = file.ModifiedAt
			summary.HasLatest = true
		}
	}

	return summary
}

// missingIndexDirectories returns eligible directories that have no index yet.
func missingIndexDirectories(indexed, eligible []string) []string {
	indexedSet := make(map[string]struct{}, len(indexed))
	for _, d := range indexed {
		indexedSet[d] = struct{}{}
	}

	missing := make([]string, 0)
	for _, d := range eligible {
		if _, ok := indexedSet[d]; !ok {
			missing = append(missing, d)
		}
	}

	return missing
}
