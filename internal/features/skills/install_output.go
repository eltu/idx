package skills

import (
	"fmt"
	"io"

	"charm.land/lipgloss/v2"
)

var (
	skillsHeaderStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#6366F1"))
	skillsEditorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#818CF8"))
	skillsStepStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#818CF8"))
	skillsTextStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#CBD5E1"))
	skillsSuccessStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#34D399"))
)

func writeHeader(w io.Writer, editor string) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  "+skillsHeaderStyle.Render("🎯 idx Skills Installer"))
	fmt.Fprintln(w, "  "+skillsEditorStyle.Render("Editor: "+displayName(editor)))
	fmt.Fprintln(w)
}

func writeStep(w io.Writer, n, total int, msg string) {
	step := skillsStepStyle.Render(fmt.Sprintf("  [%d/%d]", n, total))
	text := skillsTextStyle.Render("  " + msg)
	fmt.Fprintln(w, step+text)
}

func writeSuccess(w io.Writer, editor string) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  "+skillsSuccessStyle.Render("✓  Skills installed successfully for "+editor+"."))
	fmt.Fprintln(w, "  "+skillsTextStyle.Render("   Restart your editor to activate the new skills."))
	fmt.Fprintln(w)
}
