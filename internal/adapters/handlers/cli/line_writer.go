package cli

import (
	"fmt"
	"io"
)

type LineWriter struct {
	target io.Writer
}

// NewLineWriter adapts an io.Writer to the core output port.
// Example: writer := NewLineWriter(os.Stdout).
func NewLineWriter(target io.Writer) LineWriter {
	return LineWriter{target: target}
}

func (writer LineWriter) WriteLine(text string) error {
	_, err := fmt.Fprintln(writer.target, text)
	return err
}
