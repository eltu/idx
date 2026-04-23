package repository

import (
	"fmt"
	"os"
)

// OSFileReader reads file content from the operating system.
type OSFileReader struct {
}

// NewOSFileReader creates a new OS file reader.
func NewOSFileReader() *OSFileReader {
	return &OSFileReader{}
}

// ReadFile reads the entire contents of a file as a string.
func (reader *OSFileReader) ReadFile(path string) (string, error) {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return "", fmt.Errorf("failed to read file %q: got error %v, expected a readable file", path, err)
	}

	return string(data), nil
}
