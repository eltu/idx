package indexing

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type statusPanelData struct {
	projectRoot     string
	summary         statusSummary
	directories     []string
	configFilePath  string
	configOverrides []string
	indexStatus     string
}

func (service InitCommandService) writeStatusOverviewPanel(data statusPanelData) error {
	content := buildStatusPanelContent(service.daemonRepo, data)
	return service.output.WriteLine("\n" + statusPanelStyle.Render(content) + "\n")
}

func buildStatusPanelContent(daemonRepo ProjectMonitorChecker, data statusPanelData) string {
	projectName := filepath.Base(data.projectRoot)
	header := statusMutedStyle.Render("idx") + "  " + statusActionStyle.Render(projectName)

	rows := []string{
		header,
		"",
		panelRow("Index", data.indexStatus),
		panelRow("Files", filesCountLine(data.summary, data.directories)),
		panelRow("Updated", updatedLine(data.summary)),
		panelRow("Daemon", daemonStatusLine(daemonRepo, data.projectRoot)),
	}

	if data.configFilePath != "" {
		rows = append(rows, panelRow("Config", configPanelLine(data.configFilePath, data.configOverrides)))
	}

	return strings.Join(rows, "\n")
}

func panelRow(label, value string) string {
	return statusMutedStyle.Render(fmt.Sprintf("%-9s", label)) + value
}

func filesCountLine(summary statusSummary, directories []string) string {
	totalSize := indexTotalSizeBytes(directories)
	return fmt.Sprintf("%d files · %d directories · %s",
		summary.CheckedFiles,
		summary.CheckedDirectories,
		formatBytes(totalSize),
	)
}

func updatedLine(summary statusSummary) string {
	if !summary.HasLatest {
		return statusMutedStyle.Render("—")
	}
	age := humanAge(summary.LatestModifiedAt)
	ts := summary.LatestModifiedAt.Local().Format("15:04:05")
	return age + "  " + statusMutedStyle.Render("("+ts+")")
}

func configPanelLine(filePath string, overrides []string) string {
	name := filepath.Base(filePath)
	count := len(overrides)
	suffix := ""
	if count != 1 {
		suffix = "s"
	}
	return statusPathStyle.Render(name) + "  ·  " + statusMutedStyle.Render(fmt.Sprintf("%d override%s active", count, suffix))
}

func daemonStatusLine(daemonRepo ProjectMonitorChecker, projectRoot string) string {
	if daemonRepo == nil {
		return statusMutedStyle.Render("— not configured")
	}

	status, err := daemonRepo.ProjectStatus(projectRoot)
	if err != nil || status == nil {
		return statusMutedStyle.Render("— not configured")
	}

	if !status.Enabled {
		return statusMutedStyle.Render("⏸  disabled")
	}

	since := status.StartedAt.Local().Format("15:04")
	return statusSuccessStyle.Render("✅ watching") + "  " + statusMutedStyle.Render(fmt.Sprintf("(PID %d, since %s)", status.PID, since))
}

const humanAgeJustNow = "just now"

func humanAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return humanAgeJustNow
	case d < time.Hour:
		mins := int(d.Minutes())
		return fmt.Sprintf("%d minute%s ago", mins, pluralSuffix(mins, "", "s"))
	case d < 24*time.Hour:
		hours := int(d.Hours())
		return fmt.Sprintf("%d hour%s ago", hours, pluralSuffix(hours, "", "s"))
	case d < 7*24*time.Hour:
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%d day%s ago", days, pluralSuffix(days, "", "s"))
	default:
		return t.Local().Format("Jan 2, 2006")
	}
}

func indexTotalSizeBytes(directories []string) int64 {
	var total int64
	for _, dir := range directories {
		path := filepath.Join(dir, ".idx", "index.idx")
		if info, err := os.Stat(path); err == nil {
			total += info.Size()
		}
	}
	return total
}

func formatBytes(b int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
