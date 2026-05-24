package skills

import "fmt"

// SupportedEditors is the closed set of valid editor arguments.
var SupportedEditors = []string{"copilot", "claude", "cursor"}

// editorDisplayNames maps editor IDs to human-readable names for UI output.
var editorDisplayNames = map[string]string{
	"copilot": "GitHub Copilot",
	"claude":  "Claude Code",
	"cursor":  "Cursor",
}

func validateEditor(editor string) error {
	for _, e := range SupportedEditors {
		if e == editor {
			return nil
		}
	}
	return fmt.Errorf("unsupported editor %q: expected one of %v", editor, SupportedEditors)
}

func displayName(editor string) string {
	if name, ok := editorDisplayNames[editor]; ok {
		return name
	}
	return editor
}
