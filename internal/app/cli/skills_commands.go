package cli

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"
)

var (
	skillsCmdWarningStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FBBF24"))
	skillsCmdActionStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#6366F1"))
	skillsCmdArgStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8"))
	skillsCmdMutedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#64748B"))
	skillsCmdLabelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#818CF8"))
	skillsCmdSectionStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#CBD5E1"))
	skillsCmdDimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#475569"))
)

type skillsEditorEntry struct {
	id    string
	label string
}

var skillsEditors = []skillsEditorEntry{
	{"copilot", "GitHub Copilot"},
	{"claude", "Claude Code"},
	{"cursor", "Cursor"},
}

// skillsableCommand defines the Install method consumed by the skills command.
type skillsableCommand interface {
	Install(editor string, verbose bool) error
}

// newSkillsCommand creates the parent 'idx skills' command.
func (runner CommandRunner) newSkillsCommand() *cobra.Command {
	skillsCmd := &cobra.Command{
		Use:   "skills",
		Short: "Manage idx skills for your editor",
	}
	skillsCmd.SetHelpFunc(func(cmd *cobra.Command, _ []string) {
		fmt.Fprintln(cmd.OutOrStdout(), renderSkillsHelp())
	})
	skillsCmd.AddCommand(runner.newSkillsInstallCommand())
	return skillsCmd
}

// newSkillsInstallCommand creates 'idx skills install <editor> [--verbose]'.
func (runner CommandRunner) newSkillsInstallCommand() *cobra.Command {
	var verbose bool
	var installCmd *cobra.Command
	installCmd = &cobra.Command{
		Use:   "install <editor>",
		Short: "Install idx skills for your editor",
		Args:  cobra.ArbitraryArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				writeSkillsMissingEditorError(installCmd)
				return fmt.Errorf("")
			}
			return runner.skillsCommand.Install(args[0], verbose)
		},
	}
	installCmd.Flags().BoolVar(&verbose, "verbose", false, "Stream git and installer output")
	installCmd.SetHelpFunc(func(cmd *cobra.Command, _ []string) {
		fmt.Fprintln(cmd.OutOrStdout(), renderSkillsInstallHelp())
	})
	return installCmd
}

type skillsHelpParams struct {
	title               string
	subtitle            string
	usage               string
	usageArg            string
	footer              string
	showCommandsSection bool
}

func renderSkillsHelp() string {
	return renderSkillsHelpPage(skillsHelpParams{
		title:               "🎯 idx skills",
		subtitle:            "Install idx skills for AI-powered editors.",
		usage:               "idx skills",
		usageArg:            "<command>",
		footer:              "Run 'idx skills install --help' for more information.",
		showCommandsSection: true,
	})
}

func renderSkillsInstallHelp() string {
	return renderSkillsHelpPage(skillsHelpParams{
		title:    "🎯 idx skills install",
		subtitle: "Install idx skills from github.com/eltu/idx-skills.",
		usage:    "idx skills install",
		usageArg: "<editor>",
		footer:   "Requires git to be installed and available in $PATH.",
	})
}

func renderSkillsHelpPage(p skillsHelpParams) string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("  " + skillsCmdActionStyle.Render(p.title) + "\n")
	b.WriteString("  " + skillsCmdMutedStyle.Render(p.subtitle) + "\n")
	b.WriteString("\n")
	b.WriteString("  " + skillsCmdSectionStyle.Render("Usage") + "\n")
	b.WriteString("    " + skillsCmdMutedStyle.Render(p.usage) + " " + skillsCmdArgStyle.Render(p.usageArg) + "\n")
	b.WriteString("\n")
	if p.showCommandsSection {
		appendSkillsCommandsSection(&b)
	}
	appendSkillsEditorSection(&b)
	appendSkillsInstallExamplesSection(&b)
	b.WriteString("  " + skillsCmdDimStyle.Render(p.footer) + "\n")
	return b.String()
}

func appendSkillsCommandsSection(b *strings.Builder) {
	b.WriteString("  " + skillsCmdSectionStyle.Render("Commands") + "\n")
	b.WriteString("    " + skillsCmdActionStyle.Render("install") + " " + skillsCmdArgStyle.Render("<editor>") +
		"  " + skillsCmdMutedStyle.Render("Install idx skills for the specified editor") + "\n")
	b.WriteString("\n")
}

// appendSkillsEditorSection writes the "Editors" table section into b.
func appendSkillsEditorSection(b *strings.Builder) {
	b.WriteString("  " + skillsCmdSectionStyle.Render("Editors") + "\n")
	for _, e := range skillsEditors {
		b.WriteString("    " + renderSkillsEditorRow(e) + "\n")
	}
	b.WriteString("\n")
}

// appendSkillsInstallExamplesSection writes the "Examples" section with one install line per editor.
func appendSkillsInstallExamplesSection(b *strings.Builder) {
	b.WriteString("  " + skillsCmdSectionStyle.Render("Examples") + "\n")
	for _, e := range skillsEditors {
		b.WriteString("    " + skillsCmdMutedStyle.Render("idx skills install") + " " + skillsCmdArgStyle.Render(e.id) + "\n")
	}
	b.WriteString("\n")
}

// renderSkillsEditorRow returns the styled id+label token pair shared by editor listings.
func renderSkillsEditorRow(e skillsEditorEntry) string {
	return skillsCmdArgStyle.Render(fmt.Sprintf("%-10s", e.id)) + "  " + skillsCmdLabelStyle.Render(e.label)
}

func writeSkillsMissingEditorError(cmd *cobra.Command) {
	usage := skillsCmdActionStyle.Render("idx skills install") + " " + skillsCmdArgStyle.Render("<editor>")

	rows := make([]string, 0, len(skillsEditors))
	for _, e := range skillsEditors {
		rows = append(rows, skillsCmdMutedStyle.Render("idx skills install")+" "+renderSkillsEditorRow(e))
	}

	msg := fmt.Sprintf("\n%s\n\n  Usage:    %s\n\n  Editors:\n    %s\n",
		skillsCmdWarningStyle.Render("⚠  Missing editor argument"),
		usage,
		strings.Join(rows, "\n    "),
	)

	fmt.Fprintln(cmd.OutOrStdout(), msg)
}
