package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type checksumPayload struct {
	Files map[string]string `json:"files"`
}

// DirectoryChecksumRepository persists per-file checksums for incremental sync.
type DirectoryChecksumRepository struct {
	mu    sync.RWMutex
	cache map[string]checksumCacheEntry
}

type checksumCacheEntry struct {
	modTime time.Time
	files   map[string]string
}

// NewDirectoryChecksumRepository creates a checksum repository stored in .idx/checksum.
func NewDirectoryChecksumRepository() *DirectoryChecksumRepository {
	return &DirectoryChecksumRepository{cache: map[string]checksumCacheEntry{}}
}

// Load reads checksum data from .idx/checksum in the target directory.
func (repo *DirectoryChecksumRepository) Load(directoryPath string) (map[string]string, bool, error) {
	checksumPath := checksumFilePath(directoryPath)
	fileInfo, err := os.Stat(checksumPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			repo.deleteCacheEntry(directoryPath)
			return map[string]string{}, false, nil
		}

		return nil, false, fmt.Errorf("failed to read checksum file %q: got error %v, expected a readable file", checksumPath, err)
	}

	cachedFiles, hit := repo.cachedChecksums(directoryPath, fileInfo.ModTime())
	if hit {
		return cachedFiles, true, nil
	}

	content, err := os.ReadFile(checksumPath) //nolint:gosec
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			repo.deleteCacheEntry(directoryPath)
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

	repo.storeCacheEntry(directoryPath, fileInfo.ModTime(), payload.Files)

	return cloneChecksumMap(payload.Files), true, nil
}

// Save writes checksum data to .idx/checksum in the target directory.
func (repo *DirectoryChecksumRepository) Save(directoryPath string, checksums map[string]string) error {
	checksumPath := checksumFilePath(directoryPath)
	clonedChecksums := cloneChecksumMap(checksums)
	payload := checksumPayload{Files: clonedChecksums}

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

	fileInfo, err := os.Stat(checksumPath)
	if err != nil {
		return fmt.Errorf("failed to stat checksum file %q after write: got error %v, expected readable metadata", checksumPath, err)
	}

	repo.storeCacheEntry(directoryPath, fileInfo.ModTime(), clonedChecksums)

	return nil
}

func (repo *DirectoryChecksumRepository) cachedChecksums(directoryPath string, modTime time.Time) (map[string]string, bool) {
	repo.mu.RLock()
	entry, exists := repo.cache[directoryPath]
	repo.mu.RUnlock()
	if !exists || !entry.modTime.Equal(modTime) {
		return nil, false
	}

	return cloneChecksumMap(entry.files), true
}

func (repo *DirectoryChecksumRepository) storeCacheEntry(directoryPath string, modTime time.Time, checksums map[string]string) {
	repo.mu.Lock()
	repo.cache[directoryPath] = checksumCacheEntry{modTime: modTime, files: cloneChecksumMap(checksums)}
	repo.mu.Unlock()
}

func (repo *DirectoryChecksumRepository) deleteCacheEntry(directoryPath string) {
	repo.mu.Lock()
	delete(repo.cache, directoryPath)
	repo.mu.Unlock()
}

func cloneChecksumMap(source map[string]string) map[string]string {
	if source == nil {
		return map[string]string{}
	}

	cloned := make(map[string]string, len(source))
	for fileName, checksum := range source {
		cloned[fileName] = checksum
	}

	return cloned
}

func checksumFilePath(directoryPath string) string {
	return filepath.Join(directoryPath, ".idx", "checksum")
}
