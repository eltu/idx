package indexing

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"idx/internal/core/domain"
	"idx/internal/core/ports"
)

var (
	statusPanelStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	statusSuccessStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#34D399"))
	statusWarningStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FBBF24"))
	statusMutedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#64748B"))
	statusPathStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8"))
	statusActionStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#6366F1"))
	statusStaleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F87171"))
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

func (service InitCommandService) runStatus(profile bool) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}

	projectRoot, matcher, err := service.statusMatcher()
	if err != nil {
		return err
	}

	directories, err := IndexedDirectories(service.projectTree, projectRoot)
	if err != nil {
		return err
	}

	if len(directories) == 0 {
		return fmt.Errorf("no index found under project root %q: run idx init first", projectRoot)
	}

	eligible, err := eligibleDirectories(service.projectTree, projectRoot, matcher)
	if err != nil {
		return err
	}

	eligibleWithFiles, err := service.filterDirectoriesWithFiles(eligible, projectRoot, matcher)
	if err != nil {
		return err
	}

	missing := missingIndexDirectories(directories, eligibleWithFiles)
	if len(missing) > 0 {
		return service.writeUnindexedDirectoriesError(projectRoot, missing)
	}

	reports := make([]statusDirectoryReport, 0, len(directories))
	summary := statusSummary{CheckedDirectories: len(directories)}
	staleDirectories := make([]string, 0)

	for _, directoryPath := range directories {
		report, err := service.verifyDirectoryIndexCurrent(directoryPath, projectRoot, matcher)
		if err != nil {
			return err
		}

		reports = append(reports, report)
		summary = updateStatusSummary(summary, report)
		if report.ShouldReindex {
			staleDirectories = append(staleDirectories, directoryPath)
		}

		if !report.ShouldReindex {
			summary.UpdatedDirectories++
		}
	}

	if profile {
		if err := service.writeStatusReport(projectRoot, reports, summary); err != nil {
			return err
		}
	}

	if len(staleDirectories) > 0 {
		if !profile {
			staleCount := len(staleDirectories)
			indexLine := statusStaleStyle.Render(fmt.Sprintf("❌ %d director%s stale", staleCount, pluralSuffix(staleCount, "y", "ies"))) +
				"  — run " + statusActionStyle.Render("idx sync")
			if err := service.writeStatusOverviewPanel(statusPanelData{
				projectRoot:     projectRoot,
				summary:         summary,
				directories:     directories,
				configFilePath:  service.statusConfigFilePath,
				configOverrides: service.statusConfigOverrides,
				indexStatus:     indexLine,
			}); err != nil {
				return err
			}
			return fmt.Errorf("stale index")
		}
		return service.writeStaleIndexError(projectRoot, staleDirectories)
	}

	if profile {
		return service.output.WriteLine("\n✅ Indices are up to date.\n")
	}

	return service.writeStatusOverviewPanel(statusPanelData{
		projectRoot:     projectRoot,
		summary:         summary,
		directories:     directories,
		configFilePath:  service.statusConfigFilePath,
		configOverrides: service.statusConfigOverrides,
		indexStatus:     statusSuccessStyle.Render("✅ up to date"),
	})
}

func (service InitCommandService) statusMatcher() (string, ports.IgnoreMatcher, error) {
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

	return projectRoot, matcher, nil
}

func (service InitCommandService) verifyDirectoryIndexCurrent(directoryPath, projectRoot string, matcher ports.IgnoreMatcher) (statusDirectoryReport, error) {
	fileEntries, err := service.indexableFileEntries(directoryPath, projectRoot, matcher)
	if err != nil {
		return statusDirectoryReport{}, err
	}

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

func statusFileReports(fileEntries []domain.DirectoryEntry, changedFileNames map[string]struct{}) []statusFileReport {
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

func (service InitCommandService) writeStatusReport(projectRoot string, reports []statusDirectoryReport, summary statusSummary) error {
	for _, report := range reports {
		directoryLabel, err := filepath.Rel(projectRoot, report.Path)
		if err != nil {
			directoryLabel = report.Path
		}

		if directoryLabel == "." {
			directoryLabel = projectRoot
		}

		directoryState := "✅ updated"
		if report.ShouldReindex {
			directoryState = "❌ stale"
		}

		section := fmt.Sprintf("📂 %s\n%s\n\n%s", directoryLabel, directoryState, renderDirectoryTable(report.Files))
		if report.StructuralChange {
			section += "\n\n! note: file set changed (added/removed files) since last indexing"
		}

		if err := service.output.WriteLine("\n" + statusPanelStyle.Render(section)); err != nil {
			return err
		}
	}

	latest := "n/a"
	if summary.HasLatest {
		latest = summary.LatestModifiedAt.Format(time.RFC3339)
	}

	summarySection := fmt.Sprintf(
		"📊 Summary\n%s",
		renderSummaryTable([][2]string{
			{"directories checked", fmt.Sprintf("%d", summary.CheckedDirectories)},
			{"directories updated", fmt.Sprintf("%d", summary.UpdatedDirectories)},
			{"files checked", fmt.Sprintf("%d", summary.CheckedFiles)},
			{"files updated", fmt.Sprintf("%d", summary.UpdatedFiles)},
			{"latest file modification", latest},
		}),
	)

	return service.output.WriteLine("\n" + statusPanelStyle.Render(summarySection))
}

func renderDirectoryTable(files []statusFileReport) string {
	const fileWidth = 48

	rows := []string{}
	rows = append(rows, renderTableHeader(fileWidth))
	rows = append(rows, renderSeparator(fileWidth))

	for _, file := range files {
		state := "✗"
		if file.Updated {
			state = "✓"
		}

		rows = append(rows, fmt.Sprintf("| %-48s | %-7s | %-20s |", truncateStatusColumn(file.Name, fileWidth), state, file.ModifiedAt.Format(time.RFC3339)))
	}

	return strings.Join(rows, "\n")
}

func renderSummaryTable(rows [][2]string) string {
	maxMetric := len("metric")
	for _, row := range rows {
		if len(row[0]) > maxMetric {
			maxMetric = len(row[0])
		}
	}

	lines := []string{fmt.Sprintf("| %-*s | %-*s |", maxMetric, "metric", 26, "value")}
	lines = append(lines, fmt.Sprintf("|-%s-|-%s-|", strings.Repeat("-", maxMetric), strings.Repeat("-", 26)))
	for _, row := range rows {
		lines = append(lines, fmt.Sprintf("| %-*s | %-26s |", maxMetric, row[0], row[1]))
	}

	return strings.Join(lines, "\n")
}

func renderTableHeader(fileWidth int) string {
	return fmt.Sprintf("| %-*s | %-7s | %-20s |", fileWidth, "file", "updated", "modified_at")
}

func renderSeparator(fileWidth int) string {
	return fmt.Sprintf("|-%s-|-%s-|-%s-|", strings.Repeat("-", fileWidth), strings.Repeat("-", 7), strings.Repeat("-", 20))
}

func truncateStatusColumn(value string, maxWidth int) string {
	if len(value) <= maxWidth {
		return value
	}

	if maxWidth < 4 {
		return value[:maxWidth]
	}

	return value[:maxWidth-3] + "..."
}

func (service InitCommandService) writeUnindexedDirectoriesError(projectRoot string, missing []string) error {
	header := statusWarningStyle.Render("⚠  Index out of sync")
	count := statusMutedStyle.Render(fmt.Sprintf("%d director%s not indexed yet:", len(missing), pluralSuffix(len(missing), "y", "ies")))
	action := fmt.Sprintf("Run %s to update.", statusActionStyle.Render("idx sync"))

	lines := []string{"", header, "", count}
	for _, dir := range missing {
		rel, err := filepath.Rel(projectRoot, dir)
		if err != nil {
			rel = dir
		}
		lines = append(lines, "  "+statusPathStyle.Render(rel))
	}
	lines = append(lines, "", "  "+action, "")

	if err := service.output.WriteLine(strings.Join(lines, "\n")); err != nil {
		return err
	}

	return fmt.Errorf("unindexed directories found")
}

func (service InitCommandService) writeStaleIndexError(projectRoot string, stale []string) error {
	header := statusStaleStyle.Render("✗  Stale index detected")
	count := statusMutedStyle.Render(fmt.Sprintf("%d director%s with outdated index:", len(stale), pluralSuffix(len(stale), "y", "ies")))
	action := fmt.Sprintf("Run %s to update.", statusActionStyle.Render("idx sync"))

	lines := []string{"", header, "", count}
	for _, dir := range stale {
		rel, err := filepath.Rel(projectRoot, dir)
		if err != nil {
			rel = dir
		}
		lines = append(lines, "  "+statusPathStyle.Render(rel))
	}
	lines = append(lines, "", "  "+action, "")

	if err := service.output.WriteLine(strings.Join(lines, "\n")); err != nil {
		return err
	}

	return fmt.Errorf("stale index")
}

func pluralSuffix(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

// missingIndexDirectories returns eligible directories that have no index yet.
func missingIndexDirectories(indexed []string, eligible []string) []string {
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

// filterDirectoriesWithFiles returns only the directories that have at least one indexable file.
func (service InitCommandService) filterDirectoriesWithFiles(directories []string, projectRoot string, matcher ports.IgnoreMatcher) ([]string, error) {
	result := make([]string, 0, len(directories))
	for _, directoryPath := range directories {
		fileEntries, err := service.indexableFileEntries(directoryPath, projectRoot, matcher)
		if err != nil {
			return nil, err
		}

		if len(fileEntries) > 0 {
			result = append(result, directoryPath)
		}
	}

	return result, nil
}
