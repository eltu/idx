package cli

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"
)

var (
	configMutedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#64748B"))
	configPathStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8"))
	configWarningStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FBBF24"))
	configActionStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#6366F1"))
)

func (runner CommandRunner) newConfigCommand() *cobra.Command {
	configCommand := &cobra.Command{
		Use:   "config",
		Short: "Show or manage project configuration (.idx.yml)",
	}

	configCommand.AddCommand(runner.newConfigShowCommand())
	return configCommand
}

func (runner CommandRunner) newConfigShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Display resolved configuration and which values come from .idx.yml",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runner.writeConfigDetails()
		},
	}
}

// showConfigBanner writes a one-line config notice at the top of idx status.
// Prints nothing when no .idx.yml is present.
func (runner CommandRunner) showConfigBanner() {
	if runner.configFilePath == "" {
		return
	}

	count := len(runner.configOverrides)
	label := configMutedStyle.Render("Config")
	file := configPathStyle.Render(".idx.yml")

	overrideSuffix := ""
	if count > 0 {
		noun := "override"
		if count != 1 {
			noun = "overrides"
		}
		overrideSuffix = "  " + configWarningStyle.Render(fmt.Sprintf("%d %s active", count, noun))
	}

	fmt.Printf("\n  ⚙  %s  %s%s\n", label, file, overrideSuffix)
}

// writeConfigDetails writes the full config table for idx config show.
func (runner CommandRunner) writeConfigDetails() error {
	overrideSet := make(map[string]bool, len(runner.configOverrides))
	for _, k := range runner.configOverrides {
		overrideSet[k] = true
	}

	if runner.configFilePath == "" {
		header := configMutedStyle.Render("No .idx.yml found — using built-in defaults.")
		tip := configMutedStyle.Render("Tip: create .idx.yml at the project root to customise defaults.")
		fmt.Printf("\n  %s\n  %s\n\n", header, tip)
		return nil
	}

	fmt.Printf("\n  %s  %s\n\n",
		configMutedStyle.Render("Config"),
		configPathStyle.Render(runner.configFilePath),
	)

	rows := buildConfigRows(runner)
	maxKey := 0
	maxVal := 0
	for _, r := range rows {
		if len(r.key) > maxKey {
			maxKey = len(r.key)
		}
		if len(r.value) > maxVal {
			maxVal = len(r.value)
		}
	}

	for _, r := range rows {
		keyStyle := configMutedStyle
		sourceLabel := configMutedStyle.Render("· default")
		if overrideSet[r.key] {
			keyStyle = configActionStyle
			sourceLabel = configWarningStyle.Render("← .idx.yml") +
				configMutedStyle.Render(fmt.Sprintf("  (default: %s)", r.defaultValue))
		}

		fmt.Printf("  %s  %s  %s\n",
			keyStyle.Render(padRight(r.key, maxKey)),
			lipgloss.NewStyle().Foreground(lipgloss.Color("#E2E8F0")).Render(padRight(r.value, maxVal)),
			sourceLabel,
		)
	}
	fmt.Println()
	return nil
}

type configRow struct {
	key          string
	value        string
	defaultValue string
}

func buildConfigRows(runner CommandRunner) []configRow {
	cfg := runner.config
	def := defaultConfigValues()

	return []configRow{
		{"search.format", cfg.Search.Format, def["search.format"]},
		{"search.size", fmt.Sprintf("%d", cfg.Search.Size), def["search.size"]},
		{"search.operator", cfg.Search.Operator, def["search.operator"]},
		{"search.context", fmt.Sprintf("%d", cfg.Search.Context), def["search.context"]},
		{"search.relaxation", cfg.Search.Relaxation, def["search.relaxation"]},
		{"search.cache_ttl", cfg.Search.CacheTTL.String(), def["search.cache_ttl"]},
		{"search.max_workers", fmt.Sprintf("%d", cfg.Search.MaxWorkers), def["search.max_workers"]},
		{"watch.debounce", cfg.Watch.Debounce.String(), def["watch.debounce"]},
		{"index.ignore", formatIgnorePatterns(cfg.Index.Ignore), def["index.ignore"]},
		{"bm25.k1", formatConfigFloat(cfg.BM25.K1), def["bm25.k1"]},
		{"bm25.b", formatConfigFloat(cfg.BM25.B), def["bm25.b"]},
		{"bm25.proximity_weight", formatConfigFloat(cfg.BM25.ProximityWeight), def["bm25.proximity_weight"]},
		{"log.level", cfg.Log.Level, def["log.level"]},
	}
}

func defaultConfigValues() map[string]string {
	return map[string]string{
		"search.format":         "text",
		"search.size":           "0",
		"search.operator":       "AND",
		"search.context":        "0",
		"search.relaxation":     "",
		"search.cache_ttl":      time.Minute.String(),
		"search.max_workers":    "4",
		"watch.debounce":        (750 * time.Millisecond).String(),
		"index.ignore":          "[]",
		"bm25.k1":               "1.5",
		"bm25.b":                "0.75",
		"bm25.proximity_weight": "3",
		"log.level":             "error",
	}
}

func formatConfigFloat(f float64) string {
	return fmt.Sprintf("%.4g", f)
}

func formatIgnorePatterns(patterns []string) string {
	if len(patterns) == 0 {
		return "[]"
	}
	return "[" + strings.Join(patterns, ", ") + "]"
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
