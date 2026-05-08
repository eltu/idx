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

func TestLineWriterQuietSuppressesOutput(t *testing.T) {
	buffer := &bytes.Buffer{}
	writer := NewLineWriter(buffer)
	writer.SetQuiet(true)

	err := writer.WriteLine("should not appear")
	if err != nil {
		t.Fatalf("expected no error in quiet mode, got %v", err)
	}

	if buffer.String() != "" {
		t.Fatalf("expected empty output in quiet mode, got %q", buffer.String())
	}
}

func TestLineWriterQuietCanBeDisabled(t *testing.T) {
	buffer := &bytes.Buffer{}
	writer := NewLineWriter(buffer)

	writer.SetQuiet(true)
	_ = writer.WriteLine("suppressed")

	writer.SetQuiet(false)
	_ = writer.WriteLine("visible")

	if buffer.String() != "visible\n" {
		t.Fatalf("expected only post-quiet output, got %q", buffer.String())
	}
}
