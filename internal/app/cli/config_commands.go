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

const (
	configKeySearchFormat     = "search.format"
	configKeySearchSize       = "search.size"
	configKeySearchOperator   = "search.operator"
	configKeySearchContext    = "search.context"
	configKeySearchRelaxation = "search.relaxation"
	configKeySearchCacheTTL   = "search.cache_ttl"
	configKeySearchMaxWorkers = "search.max_workers"
	configKeyWatchDebounce    = "watch.debounce"
	configKeyIndexIgnore      = "index.ignore"
	configKeyBM25K1           = "bm25.k1"
	configKeyBM25B            = "bm25.b"
	configKeyBM25ProxWeight   = "bm25.proximity_weight"
	configKeyLogLevel         = "log.level"
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
		printConfigNoFileMessage()
		return nil
	}
	fmt.Printf("\n  %s  %s\n\n",
		configMutedStyle.Render("Config"),
		configPathStyle.Render(runner.configFilePath),
	)
	printConfigTable(buildConfigRows(runner), overrideSet)
	return nil
}

func printConfigNoFileMessage() {
	header := configMutedStyle.Render("No .idx.yml found — using built-in defaults.")
	tip := configMutedStyle.Render("Tip: create .idx.yml at the project root to customize defaults.")
	fmt.Printf("\n  %s\n  %s\n\n", header, tip)
}

func printConfigTable(rows []configRow, overrideSet map[string]bool) {
	maxKey, maxVal := configTableColumnWidths(rows)
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
}

func configTableColumnWidths(rows []configRow) (int, int) {
	maxKey, maxVal := 0, 0
	for _, r := range rows {
		if len(r.key) > maxKey {
			maxKey = len(r.key)
		}
		if len(r.value) > maxVal {
			maxVal = len(r.value)
		}
	}
	return maxKey, maxVal
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
		{configKeySearchFormat, cfg.Search.Format, def[configKeySearchFormat]},
		{configKeySearchSize, fmt.Sprintf("%d", cfg.Search.Size), def[configKeySearchSize]},
		{configKeySearchOperator, cfg.Search.Operator, def[configKeySearchOperator]},
		{configKeySearchContext, fmt.Sprintf("%d", cfg.Search.Context), def[configKeySearchContext]},
		{configKeySearchRelaxation, cfg.Search.Relaxation, def[configKeySearchRelaxation]},
		{configKeySearchCacheTTL, cfg.Search.CacheTTL.String(), def[configKeySearchCacheTTL]},
		{configKeySearchMaxWorkers, fmt.Sprintf("%d", cfg.Search.MaxWorkers), def[configKeySearchMaxWorkers]},
		{configKeyWatchDebounce, cfg.Watch.Debounce.String(), def[configKeyWatchDebounce]},
		{configKeyIndexIgnore, formatIgnorePatterns(cfg.Index.Ignore), def[configKeyIndexIgnore]},
		{configKeyBM25K1, formatConfigFloat(cfg.BM25.K1), def[configKeyBM25K1]},
		{configKeyBM25B, formatConfigFloat(cfg.BM25.B), def[configKeyBM25B]},
		{configKeyBM25ProxWeight, formatConfigFloat(cfg.BM25.ProximityWeight), def[configKeyBM25ProxWeight]},
		{configKeyLogLevel, cfg.Log.Level, def[configKeyLogLevel]},
	}
}

func defaultConfigValues() map[string]string {
	return map[string]string{
		configKeySearchFormat:     "text",
		configKeySearchSize:       "0",
		configKeySearchOperator:   "AND",
		configKeySearchContext:    "0",
		configKeySearchRelaxation: "",
		configKeySearchCacheTTL:   time.Minute.String(),
		configKeySearchMaxWorkers: "4",
		configKeyWatchDebounce:    (750 * time.Millisecond).String(),
		configKeyIndexIgnore:      "[]",
		configKeyBM25K1:           "1.5",
		configKeyBM25B:            "0.75",
		configKeyBM25ProxWeight:   "3",
		configKeyLogLevel:         "error",
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
