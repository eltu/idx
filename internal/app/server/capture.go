package server

import "strings"

// captureWriter collects lines written by idx services for structured extraction.
type captureWriter struct{ lines []string }

func (w *captureWriter) WriteLine(text string) error {
	w.lines = append(w.lines, text)
	return nil
}

func (w *captureWriter) joined() string {
	return strings.Join(w.lines, "\n")
}

func (w *captureWriter) firstLine() string {
	if len(w.lines) == 0 {
		return ""
	}
	return w.lines[0]
}
