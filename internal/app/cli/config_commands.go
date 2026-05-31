package cli

import (
	"fmt"
	"io"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"

	sharedconfig "idx/internal/shared/config"
)

var (
	configMutedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#64748B"))
	configPathStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8"))
	configWarningStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FBBF24"))
	configActionStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#6366F1"))
)

// ConfigShower is the single capability required to display the resolved configuration.
type ConfigShower interface {
	Show() error
}

func (runner CommandRunner) newConfigCommand() *cobra.Command {
	configCommand := &cobra.Command{
		Use:   "config",
		Short: "Show or manage project configuration (.idx.yml)",
	}
	configCommand.AddCommand(runner.newConfigShowCommand(), runner.newConfigGetCommand())
	return configCommand
}

func (runner CommandRunner) newConfigGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get the resolved value of a single config key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runner.runConfigGetTo(cmd.OutOrStdout(), args[0])
		},
	}
}

// runConfigGetTo writes the resolved value of key to out.
// Returns an error when key is not a recognized configuration field.
// Example: runner.runConfigGetTo(buf, "search.operator") → writes "AND\n" to buf.
func (runner CommandRunner) runConfigGetTo(out io.Writer, key string) error {
	value := sharedconfig.FieldValue(runner.config, key)
	if value == "" {
		keys := sharedconfig.AllKeys()
		return fmt.Errorf("unknown config key %q — valid keys: %s", key, strings.Join(keys, ", "))
	}
	_, err := fmt.Fprintln(out, value)
	return err
}

func (runner CommandRunner) newConfigShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Display resolved configuration and which values come from .idx.yml",
		RunE: func(_ *cobra.Command, _ []string) error {
			if runner.configCommand == nil {
				// fallback for server-mode or unit-test contexts without RPC wiring
				return runner.writeConfigDetails()
			}
			return runner.configCommand.Show()
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

// writeConfigDetails writes the styled config table directly to os.Stdout.
// Used as a fallback when configCommand is nil and by showConfigBanner tests.
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
			keyStyle.Render(sharedconfig.PadRight(r.key, maxKey)),
			lipgloss.NewStyle().Foreground(lipgloss.Color("#E2E8F0")).Render(sharedconfig.PadRight(r.value, maxVal)),
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
	keys := sharedconfig.AllKeys()
	rows := make([]configRow, 0, len(keys))
	for _, key := range keys {
		rows = append(rows, configRow{
			key:          key,
			value:        sharedconfig.FieldValue(cfg, key),
			defaultValue: sharedconfig.DefaultFieldValue(key),
		})
	}
	return rows
}
