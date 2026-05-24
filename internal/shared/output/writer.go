package output

import (
	"fmt"
	"io"
)

// LineWriter adapts an io.Writer to the output.Writer port.
// Example: w := NewLineWriter(os.Stdout).
type LineWriter struct {
	target io.Writer
	quiet  bool
}

func NewLineWriter(target io.Writer) *LineWriter {
	return &LineWriter{target: target}
}

// SetQuiet suppresses all WriteLine output when enabled.
// Use with --quiet flag to prevent informational messages from entering
// the agent context window during automated benchmark and scripted sessions.
func (w *LineWriter) SetQuiet(quiet bool) {
	w.quiet = quiet
}

func (w *LineWriter) WriteLine(text string) error {
	if w.quiet {
		return nil
	}
	_, err := fmt.Fprintln(w.target, text)
	return err
}

// WriteInline writes text without a trailing newline, allowing \r to overwrite the line.
// Used by spinner animations to update the same terminal line in place.
func (w *LineWriter) WriteInline(text string) error {
	if w.quiet {
		return nil
	}
	_, err := fmt.Fprint(w.target, text)
	return err
}
