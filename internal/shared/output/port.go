package output

// Writer is the text output port — the single way any service writes user-facing lines.
// Example: writer.WriteLine("✓ index built").
type Writer interface {
	WriteLine(text string) error
}
