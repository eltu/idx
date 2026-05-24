package storage

import "path/filepath"

// indexFilePath returns the standard index file path for a directory.
func indexFilePath(directoryPath string) string {
	return filepath.Join(directoryPath, ".idx", "index.idx")
}
