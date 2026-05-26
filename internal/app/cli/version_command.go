package cli

import (
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"
)

// idxLogo is the uppercase ASCII art for "IDX".
const idxLogo = ` ██╗██████╗ ██╗  ██╗
 ██║██╔══██╗╚██╗██╔╝
 ██║██║  ██║ ╚███╔╝
 ██║██║  ██║ ██╔██╗
 ██║██████╔╝██╔╝ ██╗
 ╚═╝╚═════╝ ╚═╝  ╚═╝`

var (
	versionLogoStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#6366F1"))
	versionNameStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F8FAFC"))
	versionTaglineStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#64748B"))
	versionValueStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FBBF24"))
	versionDateStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8"))
)

// newVersionCommand prints the binary version and build date.
// Example: idx version.
func (runner CommandRunner) newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version and build information",
		Run: func(cmd *cobra.Command, _ []string) {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), renderVersionOutput(runner.buildInfo.Version, runner.buildInfo.BuildDate))
		},
	}
}

func renderVersionOutput(version, buildDate string) string {
	logoLines := strings.Split(idxLogo, "\n")

	nameVer := versionNameStyle.Render("IDX") + " " + versionValueStyle.Render(version)
	tagline := versionTaglineStyle.Render("fast BM25 code search")
	built := versionDateStyle.Render(formatVersionDate(buildDate))
	path := versionDateStyle.Render(currentDirTilde())

	// Right-side content starts at logo line 1 to align visually with the middle.
	rightContent := []string{"", nameVer, tagline, built, path}

	maxWidth := 0
	for _, line := range logoLines {
		if w := utf8.RuneCountInString(line); w > maxWidth {
			maxWidth = w
		}
	}

	rows := make([]string, len(logoLines))
	for i, line := range logoLines {
		padded := line + strings.Repeat(" ", maxWidth-utf8.RuneCountInString(line))
		rows[i] = versionLogoStyle.Render(padded)
		if i < len(rightContent) && rightContent[i] != "" {
			rows[i] += "   " + rightContent[i]
		}
	}

	return "\n" + strings.Join(rows, "\n") + "\n"
}

func currentDirTilde() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}

	home, err := os.UserHomeDir()
	if err != nil || !strings.HasPrefix(cwd, home) {
		return cwd
	}

	return "~" + cwd[len(home):]
}

func formatVersionDate(buildDate string) string {
	t, err := time.Parse(time.RFC3339, buildDate)
	if err != nil {
		return buildDate
	}

	return t.UTC().Format("2006-01-02  15:04 UTC")
}
