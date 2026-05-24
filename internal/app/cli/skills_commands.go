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

func renderSkillsHelp() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString("  " + skillsCmdActionStyle.Render("🎯 idx skills") + "\n")
	b.WriteString("  " + skillsCmdMutedStyle.Render("Install idx skills for AI-powered editors.") + "\n")
	b.WriteString("\n")

	b.WriteString("  " + skillsCmdSectionStyle.Render("Usage") + "\n")
	b.WriteString("    " + skillsCmdMutedStyle.Render("idx skills") + " " + skillsCmdArgStyle.Render("<command>") + "\n")
	b.WriteString("\n")

	b.WriteString("  " + skillsCmdSectionStyle.Render("Commands") + "\n")
	b.WriteString("    " + skillsCmdActionStyle.Render("install") + " " + skillsCmdArgStyle.Render("<editor>") +
		"  " + skillsCmdMutedStyle.Render("Install idx skills for the specified editor") + "\n")
	b.WriteString("\n")

	b.WriteString("  " + skillsCmdSectionStyle.Render("Editors") + "\n")
	for _, e := range skillsEditors {
		b.WriteString("    " + skillsCmdArgStyle.Render(fmt.Sprintf("%-10s", e.id)) +
			"  " + skillsCmdLabelStyle.Render(e.label) + "\n")
	}
	b.WriteString("\n")

	b.WriteString("  " + skillsCmdSectionStyle.Render("Examples") + "\n")
	for _, e := range skillsEditors {
		b.WriteString("    " + skillsCmdMutedStyle.Render("idx skills install") + " " + skillsCmdArgStyle.Render(e.id) + "\n")
	}
	b.WriteString("\n")

	b.WriteString("  " + skillsCmdDimStyle.Render("Run 'idx skills install --help' for more information.") + "\n")

	return b.String()
}

func renderSkillsInstallHelp() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString("  " + skillsCmdActionStyle.Render("🎯 idx skills install") + "\n")
	b.WriteString("  " + skillsCmdMutedStyle.Render("Install idx skills from github.com/eltu/idx-skills.") + "\n")
	b.WriteString("\n")

	b.WriteString("  " + skillsCmdSectionStyle.Render("Usage") + "\n")
	b.WriteString("    " + skillsCmdMutedStyle.Render("idx skills install") + " " + skillsCmdArgStyle.Render("<editor>") + "\n")
	b.WriteString("\n")

	b.WriteString("  " + skillsCmdSectionStyle.Render("Editors") + "\n")
	for _, e := range skillsEditors {
		b.WriteString("    " + skillsCmdArgStyle.Render(fmt.Sprintf("%-10s", e.id)) +
			"  " + skillsCmdLabelStyle.Render(e.label) + "\n")
	}
	b.WriteString("\n")

	b.WriteString("  " + skillsCmdSectionStyle.Render("Examples") + "\n")
	for _, e := range skillsEditors {
		b.WriteString("    " + skillsCmdMutedStyle.Render("idx skills install") + " " + skillsCmdArgStyle.Render(e.id) + "\n")
	}
	b.WriteString("\n")

	b.WriteString("  " + skillsCmdDimStyle.Render("Requires git to be installed and available in $PATH.") + "\n")

	return b.String()
}

func writeSkillsMissingEditorError(cmd *cobra.Command) {
	usage := skillsCmdActionStyle.Render("idx skills install") + " " + skillsCmdArgStyle.Render("<editor>")

	examples := make([]string, 0, len(skillsEditors))
	for _, e := range skillsEditors {
		line := skillsCmdMutedStyle.Render("idx skills install") +
			" " + skillsCmdArgStyle.Render(fmt.Sprintf("%-10s", e.id)) +
			"  " + skillsCmdLabelStyle.Render(e.label)
		examples = append(examples, line)
	}

	msg := fmt.Sprintf("\n%s\n\n  Usage:    %s\n\n  Editors:\n    %s\n",
		skillsCmdWarningStyle.Render("⚠  Missing editor argument"),
		usage,
		strings.Join(examples, "\n    "),
	)

	fmt.Fprintln(cmd.OutOrStdout(), msg)
}
