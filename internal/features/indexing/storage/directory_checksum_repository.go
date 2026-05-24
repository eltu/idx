package storage

import (
	"idx/internal/features/indexing"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	
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
	files   map[string]indexing.FileChecksumState
}

// NewDirectoryChecksumRepository creates a checksum repository stored in .idx/checksum.idx.
func NewDirectoryChecksumRepository() *DirectoryChecksumRepository {
	return &DirectoryChecksumRepository{cache: map[string]checksumCacheEntry{}}
}

// Load reads checksum data from .idx/checksum.idx in the target directory.
func (repo *DirectoryChecksumRepository) Load(directoryPath string) (map[string]string, bool, error) {
	snapshot, exists, err := repo.LoadSnapshot(directoryPath)
	if err != nil {
		return nil, false, err
	}

	return snapshotToChecksums(snapshot), exists, nil
}

// LoadSnapshot reads checksum snapshot data from .idx/checksum.idx in the target directory.
func (repo *DirectoryChecksumRepository) LoadSnapshot(directoryPath string) (indexing.DirectoryChecksumSnapshot, bool, error) {
	checksumPath := checksumFilePath(directoryPath)
	fileInfo, err := os.Stat(checksumPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			repo.deleteCacheEntry(directoryPath)
			return indexing.DirectoryChecksumSnapshot{Files: map[string]indexing.FileChecksumState{}}, false, nil
		}

		return indexing.DirectoryChecksumSnapshot{}, false, fmt.Errorf("failed to read checksum file %q: got error %v, expected a readable file", checksumPath, err)
	}

	cachedFiles, hit := repo.cachedChecksums(directoryPath, fileInfo.ModTime())
	if hit {
		return indexing.DirectoryChecksumSnapshot{Files: cachedFiles}, true, nil
	}

	content, err := os.ReadFile(checksumPath) //nolint:gosec
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			repo.deleteCacheEntry(directoryPath)
			return indexing.DirectoryChecksumSnapshot{Files: map[string]indexing.FileChecksumState{}}, false, nil
		}

		return indexing.DirectoryChecksumSnapshot{}, false, fmt.Errorf("failed to read checksum file %q: got error %v, expected a readable file", checksumPath, err)
	}

	var payload checksumPayload
	if err := json.Unmarshal(content, &payload); err != nil {
		return indexing.DirectoryChecksumSnapshot{}, false, fmt.Errorf("failed to parse checksum file %q: got error %v, expected valid JSON payload", checksumPath, err)
	}

	snapshot := payloadToSnapshot(payload)

	repo.storeCacheEntry(directoryPath, fileInfo.ModTime(), snapshot.Files)

	return indexing.DirectoryChecksumSnapshot{Files: cloneSnapshotFiles(snapshot.Files)}, true, nil
}

// Save writes checksum data to .idx/checksum.idx in the target directory.
func (repo *DirectoryChecksumRepository) Save(directoryPath string, checksums map[string]string) error {
	files := make(map[string]indexing.FileChecksumState, len(checksums))
	for fileName, checksum := range checksums {
		files[fileName] = indexing.FileChecksumState{Checksum: checksum}
	}

	return repo.SaveSnapshot(directoryPath, indexing.DirectoryChecksumSnapshot{Files: files})
}

// SaveSnapshot writes checksum snapshot data to .idx/checksum.idx in the target directory.
func (repo *DirectoryChecksumRepository) SaveSnapshot(directoryPath string, snapshot indexing.DirectoryChecksumSnapshot) error {
	checksumPath := checksumFilePath(directoryPath)
	clonedSnapshot := indexing.DirectoryChecksumSnapshot{Files: cloneSnapshotFiles(snapshot.Files)}
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

func (repo *DirectoryChecksumRepository) cachedChecksums(directoryPath string, modTime time.Time) (map[string]indexing.FileChecksumState, bool) {
	repo.mu.RLock()
	entry, exists := repo.cache[directoryPath]
	repo.mu.RUnlock()
	if !exists || !entry.modTime.Equal(modTime) {
		return nil, false
	}

	return cloneSnapshotFiles(entry.files), true
}

func (repo *DirectoryChecksumRepository) storeCacheEntry(directoryPath string, modTime time.Time, checksums map[string]indexing.FileChecksumState) {
	repo.mu.Lock()
	repo.cache[directoryPath] = checksumCacheEntry{modTime: modTime, files: cloneSnapshotFiles(checksums)}
	repo.mu.Unlock()
}

func (repo *DirectoryChecksumRepository) deleteCacheEntry(directoryPath string) {
	repo.mu.Lock()
	delete(repo.cache, directoryPath)
	repo.mu.Unlock()
}

func snapshotToChecksums(snapshot indexing.DirectoryChecksumSnapshot) map[string]string {
	checksums := make(map[string]string, len(snapshot.Files))
	for fileName, state := range snapshot.Files {
		checksums[fileName] = state.Checksum
	}

	return checksums
}

func cloneSnapshotFiles(source map[string]indexing.FileChecksumState) map[string]indexing.FileChecksumState {
	if source == nil {
		return map[string]indexing.FileChecksumState{}
	}

	cloned := make(map[string]indexing.FileChecksumState, len(source))
	for fileName, state := range source {
		cloned[fileName] = state
	}

	return cloned
}

func payloadToSnapshot(payload checksumPayload) indexing.DirectoryChecksumSnapshot {
	if len(payload.FileStates) > 0 {
		files := make(map[string]indexing.FileChecksumState, len(payload.FileStates))
		for fileName, state := range payload.FileStates {
			files[fileName] = indexing.FileChecksumState{
				Checksum:        state.Checksum,
				Size:            state.Size,
				ModTimeUnixNano: state.ModTimeUnixNano,
			}
		}

		return indexing.DirectoryChecksumSnapshot{Files: files}
	}

	files := make(map[string]indexing.FileChecksumState, len(payload.Files))
	for fileName, checksum := range payload.Files {
		files[fileName] = indexing.FileChecksumState{Checksum: checksum}
	}

	return indexing.DirectoryChecksumSnapshot{Files: files}
}

func snapshotToPayload(snapshot indexing.DirectoryChecksumSnapshot) checksumPayload {
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
	return filepath.Join(directoryPath, ".idx", "checksum.idx")
}
