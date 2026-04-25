package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type checksumPayload struct {
	Files map[string]string `json:"files"`
}

// DirectoryChecksumRepository persists per-file checksums for incremental sync.
type DirectoryChecksumRepository struct{}

// NewDirectoryChecksumRepository creates a checksum repository stored in .idx/checksum.
func NewDirectoryChecksumRepository() *DirectoryChecksumRepository {
	return &DirectoryChecksumRepository{}
}

// Load reads checksum data from .idx/checksum in the target directory.
func (repo *DirectoryChecksumRepository) Load(directoryPath string) (map[string]string, bool, error) {
	checksumPath := checksumFilePath(directoryPath)
	content, err := os.ReadFile(checksumPath) //nolint:gosec
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]string{}, false, nil
		}

		return nil, false, fmt.Errorf("failed to read checksum file %q: got error %v, expected a readable file", checksumPath, err)
	}

	var payload checksumPayload
	if err := json.Unmarshal(content, &payload); err != nil {
		return nil, false, fmt.Errorf("failed to parse checksum file %q: got error %v, expected valid JSON payload", checksumPath, err)
	}

	if payload.Files == nil {
		payload.Files = map[string]string{}
	}

	return payload.Files, true, nil
}

// Save writes checksum data to .idx/checksum in the target directory.
func (repo *DirectoryChecksumRepository) Save(directoryPath string, checksums map[string]string) error {
	checksumPath := checksumFilePath(directoryPath)
	payload := checksumPayload{Files: checksums}

	content, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to encode checksum payload for %q: got error %v, expected a serializable map", directoryPath, err)
	}

	if err := os.MkdirAll(filepath.Dir(checksumPath), 0750); err != nil {
		return fmt.Errorf("failed to create checksum directory for %q: got error %v, expected writable path", checksumPath, err)
	}

	if err := os.WriteFile(checksumPath, content, 0600); err != nil {
		return fmt.Errorf("failed to write checksum file %q: got error %v, expected writable path", checksumPath, err)
	}

	return nil
}

func checksumFilePath(directoryPath string) string {
	return filepath.Join(directoryPath, ".idx", "checksum")
}
