package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"idx/internal/core/ports"
)

type checksumPayload struct {
	Files      map[string]string            `json:"files,omitempty"`
	FileStates map[string]checksumFileState `json:"fileStates,omitempty"`
}

type checksumFileState struct {
	Checksum        string `json:"checksum"`
	Size            int64  `json:"size"`
	ModTimeUnixNano int64  `json:"modTimeUnixNano"`
}

// DirectoryChecksumRepository persists per-file checksums for incremental sync.
type DirectoryChecksumRepository struct {
	mu    sync.RWMutex
	cache map[string]checksumCacheEntry
}

type checksumCacheEntry struct {
	modTime time.Time
	files   map[string]ports.FileChecksumState
}

// NewDirectoryChecksumRepository creates a checksum repository stored in .idx/checksum.
func NewDirectoryChecksumRepository() *DirectoryChecksumRepository {
	return &DirectoryChecksumRepository{cache: map[string]checksumCacheEntry{}}
}

// Load reads checksum data from .idx/checksum in the target directory.
func (repo *DirectoryChecksumRepository) Load(directoryPath string) (map[string]string, bool, error) {
	snapshot, exists, err := repo.LoadSnapshot(directoryPath)
	if err != nil {
		return nil, false, err
	}

	return snapshotToChecksums(snapshot), exists, nil
}

// LoadSnapshot reads checksum snapshot data from .idx/checksum in the target directory.
func (repo *DirectoryChecksumRepository) LoadSnapshot(directoryPath string) (ports.DirectoryChecksumSnapshot, bool, error) {
	checksumPath := checksumFilePath(directoryPath)
	fileInfo, err := os.Stat(checksumPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			repo.deleteCacheEntry(directoryPath)
			return ports.DirectoryChecksumSnapshot{Files: map[string]ports.FileChecksumState{}}, false, nil
		}

		return ports.DirectoryChecksumSnapshot{}, false, fmt.Errorf("failed to read checksum file %q: got error %v, expected a readable file", checksumPath, err)
	}

	cachedFiles, hit := repo.cachedChecksums(directoryPath, fileInfo.ModTime())
	if hit {
		return ports.DirectoryChecksumSnapshot{Files: cachedFiles}, true, nil
	}

	content, err := os.ReadFile(checksumPath) //nolint:gosec
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			repo.deleteCacheEntry(directoryPath)
			return ports.DirectoryChecksumSnapshot{Files: map[string]ports.FileChecksumState{}}, false, nil
		}

		return ports.DirectoryChecksumSnapshot{}, false, fmt.Errorf("failed to read checksum file %q: got error %v, expected a readable file", checksumPath, err)
	}

	var payload checksumPayload
	if err := json.Unmarshal(content, &payload); err != nil {
		return ports.DirectoryChecksumSnapshot{}, false, fmt.Errorf("failed to parse checksum file %q: got error %v, expected valid JSON payload", checksumPath, err)
	}

	snapshot := payloadToSnapshot(payload)

	repo.storeCacheEntry(directoryPath, fileInfo.ModTime(), snapshot.Files)

	return ports.DirectoryChecksumSnapshot{Files: cloneSnapshotFiles(snapshot.Files)}, true, nil
}

// Save writes checksum data to .idx/checksum in the target directory.
func (repo *DirectoryChecksumRepository) Save(directoryPath string, checksums map[string]string) error {
	files := make(map[string]ports.FileChecksumState, len(checksums))
	for fileName, checksum := range checksums {
		files[fileName] = ports.FileChecksumState{Checksum: checksum}
	}

	return repo.SaveSnapshot(directoryPath, ports.DirectoryChecksumSnapshot{Files: files})
}

// SaveSnapshot writes checksum snapshot data to .idx/checksum in the target directory.
func (repo *DirectoryChecksumRepository) SaveSnapshot(directoryPath string, snapshot ports.DirectoryChecksumSnapshot) error {
	checksumPath := checksumFilePath(directoryPath)
	clonedSnapshot := ports.DirectoryChecksumSnapshot{Files: cloneSnapshotFiles(snapshot.Files)}
	payload := snapshotToPayload(clonedSnapshot)

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

	repo.storeCacheEntry(directoryPath, fileInfo.ModTime(), clonedSnapshot.Files)

	return nil
}

func (repo *DirectoryChecksumRepository) cachedChecksums(directoryPath string, modTime time.Time) (map[string]ports.FileChecksumState, bool) {
	repo.mu.RLock()
	entry, exists := repo.cache[directoryPath]
	repo.mu.RUnlock()
	if !exists || !entry.modTime.Equal(modTime) {
		return nil, false
	}

	return cloneSnapshotFiles(entry.files), true
}

func (repo *DirectoryChecksumRepository) storeCacheEntry(directoryPath string, modTime time.Time, checksums map[string]ports.FileChecksumState) {
	repo.mu.Lock()
	repo.cache[directoryPath] = checksumCacheEntry{modTime: modTime, files: cloneSnapshotFiles(checksums)}
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

func snapshotToChecksums(snapshot ports.DirectoryChecksumSnapshot) map[string]string {
	checksums := make(map[string]string, len(snapshot.Files))
	for fileName, state := range snapshot.Files {
		checksums[fileName] = state.Checksum
	}

	return checksums
}

func cloneSnapshotFiles(source map[string]ports.FileChecksumState) map[string]ports.FileChecksumState {
	if source == nil {
		return map[string]ports.FileChecksumState{}
	}

	cloned := make(map[string]ports.FileChecksumState, len(source))
	for fileName, state := range source {
		cloned[fileName] = state
	}

	return cloned
}

func payloadToSnapshot(payload checksumPayload) ports.DirectoryChecksumSnapshot {
	if len(payload.FileStates) > 0 {
		files := make(map[string]ports.FileChecksumState, len(payload.FileStates))
		for fileName, state := range payload.FileStates {
			files[fileName] = ports.FileChecksumState{
				Checksum:        state.Checksum,
				Size:            state.Size,
				ModTimeUnixNano: state.ModTimeUnixNano,
			}
		}

		return ports.DirectoryChecksumSnapshot{Files: files}
	}

	files := make(map[string]ports.FileChecksumState, len(payload.Files))
	for fileName, checksum := range payload.Files {
		files[fileName] = ports.FileChecksumState{Checksum: checksum}
	}

	return ports.DirectoryChecksumSnapshot{Files: files}
}

func snapshotToPayload(snapshot ports.DirectoryChecksumSnapshot) checksumPayload {
	fileStates := make(map[string]checksumFileState, len(snapshot.Files))
	files := make(map[string]string, len(snapshot.Files))
	for fileName, state := range snapshot.Files {
		fileStates[fileName] = checksumFileState{
			Checksum:        state.Checksum,
			Size:            state.Size,
			ModTimeUnixNano: state.ModTimeUnixNano,
		}
		files[fileName] = state.Checksum
	}

	return checksumPayload{Files: files, FileStates: fileStates}
}

func checksumFilePath(directoryPath string) string {
	return filepath.Join(directoryPath, ".idx", "checksum")
}
