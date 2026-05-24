package read

import (
	"io"
	"os"
)

// OSFileStreamer opens files for streaming and reports whether a path is a directory.
type OSFileStreamer struct{}

// NewOSFileStreamer creates a filesystem-backed file streamer.
// Example: streamer := NewOSFileStreamer().
func NewOSFileStreamer() *OSFileStreamer {
	return &OSFileStreamer{}
}

// OpenFile opens the file at path for sequential reading.
// The caller is responsible for closing the returned ReadCloser.
func (OSFileStreamer) OpenFile(path string) (io.ReadCloser, error) {
	return os.Open(path) //nolint:gosec
}

// IsDir reports whether path refers to a directory.
func (OSFileStreamer) IsDir(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return info.IsDir(), nil
}
