package cli

import (
	"fmt"
	"io"
)

type LineWriter struct {
	target io.Writer
	quiet  bool
}

// NewLineWriter adapts an io.Writer to the core output port.
// Example: writer := NewLineWriter(os.Stdout).
func NewLineWriter(target io.Writer) *LineWriter {
	return &LineWriter{target: target}
}

// SetQuiet suppresses all WriteLine output when enabled.
// Use with --quiet flag to prevent informational messages from entering
// the agent context window during automated benchmark and scripted sessions.
func (writer *LineWriter) SetQuiet(quiet bool) {
	writer.quiet = quiet
}

func (writer *LineWriter) WriteLine(text string) error {
	if writer.quiet {
		return nil
	}
	_, err := fmt.Fprintln(writer.target, text)
	return err
}

// WriteInline writes text without a trailing newline, allowing \r to overwrite the line.
// Used by spinner animations to update the same terminal line in place.
func (writer *LineWriter) WriteInline(text string) error {
	if writer.quiet {
		return nil
	}
	_, err := fmt.Fprint(writer.target, text)
	return err
}
