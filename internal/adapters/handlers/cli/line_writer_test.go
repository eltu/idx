package cli

import (
	"bytes"
	"testing"
)

func TestLineWriterWriteLineAppendsNewline(t *testing.T) {
	buffer := &bytes.Buffer{}
	writer := NewLineWriter(buffer)

	err := writer.WriteLine("hello")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if buffer.String() != "hello\n" {
		t.Fatalf("expected output with newline, got %q", buffer.String())
	}
}
