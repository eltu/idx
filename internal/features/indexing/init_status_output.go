package indexing

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"
)

const (
	idxSyncCmdName        = "idx sync"
	idxInitCmdName        = "idx init"
	statusTimestampLayout = "2006-01-02T15:04:05Z07:00"
	errMsgStaleIndex      = "stale index"
	runCmdToUpdateFmt     = "Run %s to update."
)

var (
	statusPanelStyle         = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	statusSuccessStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#34D399"))
	statusWarningStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FBBF24"))
	statusMutedStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#64748B"))
	statusPathStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8"))
	statusActionStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#6366F1"))
	statusStaleStyle         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F87171"))
	statusCheckingLabelStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#818CF8"))
	statusSpinnerStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#6366F1"))
)

func (service InitCommandService) writeStatusResult(profile bool, projectRoot string, directories []string, reports []statusDirectoryReport, summary statusSummary, staleDirectories []string) error {
	if profile {
		if err := service.writeStatusReport(projectRoot, reports, summary); err != nil {
			return err
		}
	}

	if len(staleDirectories) > 0 {
		return service.writeStaleResult(profile, projectRoot, directories, summary, staleDirectories)
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

func (service InitCommandService) writeStaleResult(profile bool, projectRoot string, directories []string, summary statusSummary, staleDirectories []string) error {
	if profile {
		return service.writeStaleIndexError(projectRoot, staleDirectories)
	}

	staleCount := len(staleDirectories)
	indexLine := statusStaleStyle.Render(fmt.Sprintf("❌ %d director%s stale", staleCount, pluralSuffix(staleCount, "y", "ies"))) +
		"  — run " + statusActionStyle.Render(idxSyncCmdName)

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

	return errors.New(errMsgStaleIndex)
}

func (service InitCommandService) writeStatusReport(projectRoot string, reports []statusDirectoryReport, summary statusSummary) error {
	if err := service.writeDirectoryReports(projectRoot, reports); err != nil {
		return err
	}
	return service.writeStatusSummaryPanel(summary)
}

func (service InitCommandService) writeDirectoryReports(projectRoot string, reports []statusDirectoryReport) error {
	for _, report := range reports {
		if err := service.writeDirectoryReport(projectRoot, report); err != nil {
			return err
		}
	}
	return nil
}

func (service InitCommandService) writeDirectoryReport(projectRoot string, report statusDirectoryReport) error {
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
	return service.output.WriteLine("\n" + statusPanelStyle.Render(section))
}

func (service InitCommandService) writeStatusSummaryPanel(summary statusSummary) error {
	latest := "n/a"
	if summary.HasLatest {
		latest = summary.LatestModifiedAt.Format(statusTimestampLayout)
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

		rows = append(rows, fmt.Sprintf("| %-48s | %-7s | %-20s |", truncateStatusColumn(file.Name, fileWidth), state, file.ModifiedAt.Format(statusTimestampLayout)))
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

func (service InitCommandService) writeNotGitRepoError(currentDir string) error {
	header := statusWarningStyle.Render("⚠  Not a git repository")
	detail := statusMutedStyle.Render("idx requires a git project to locate the repository root.")
	path := statusPathStyle.Render(currentDir)
	action := fmt.Sprintf("Navigate to a git project and run %s.", statusActionStyle.Render(idxInitCmdName))

	lines := []string{"", header, "", "  " + detail, "  " + path, "", "  " + action, ""}

	if err := service.output.WriteLine(strings.Join(lines, "\n")); err != nil {
		return err
	}
	return reportedError{}
}

func (service InitCommandService) writeNoIndexError(projectRoot string) error {
	header := statusWarningStyle.Render("⚠  No index found")
	detail := statusMutedStyle.Render("This project has not been indexed yet.")
	path := statusPathStyle.Render(projectRoot)
	action := fmt.Sprintf("Run %s to get started.", statusActionStyle.Render(idxInitCmdName))

	lines := []string{"", header, "", "  " + detail, "  " + path, "", "  " + action, ""}

	if err := service.output.WriteLine(strings.Join(lines, "\n")); err != nil {
		return err
	}
	return reportedError{}
}

// writeDirListError writes a diagnostic block listing directory paths and returns finalErr.
// header and count must be pre-rendered styled strings.
// Example: service.writeDirListError(root, header, count, dirs, errors.New("stale index")).
func (service InitCommandService) writeDirListError(projectRoot, header, count string, dirs []string, finalErr error) error {
	action := fmt.Sprintf(runCmdToUpdateFmt, statusActionStyle.Render(idxSyncCmdName))
	lines := []string{"", header, "", count}
	for _, dir := range dirs {
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
	return finalErr
}

func (service InitCommandService) writeUnindexedDirectoriesError(projectRoot string, missing []string) error {
	header := statusWarningStyle.Render("⚠  Index out of sync")
	count := statusMutedStyle.Render(fmt.Sprintf("%d director%s not indexed yet:", len(missing), pluralSuffix(len(missing), "y", "ies")))
	return service.writeDirListError(projectRoot, header, count, missing, fmt.Errorf("unindexed directories found"))
}

func (service InitCommandService) writeStaleIndexError(projectRoot string, stale []string) error {
	header := statusStaleStyle.Render("✗  Stale index detected")
	count := statusMutedStyle.Render(fmt.Sprintf("%d director%s with outdated index:", len(stale), pluralSuffix(len(stale), "y", "ies")))
	return service.writeDirListError(projectRoot, header, count, stale, errors.New(errMsgStaleIndex))
}

// reportedError is returned after a formatted diagnostic has been written to output.
// Its empty Error() prevents main from printing a duplicate line to stderr while
// still causing a non-zero exit code.
type reportedError struct{}

func (reportedError) Error() string { return "" }

func pluralSuffix(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}
