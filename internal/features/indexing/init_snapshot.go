package indexing

import (
	"idx/internal/shared/filesystem"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

)

func (service InitCommandService) hasDirectoryIndex(directoryPath string) (bool, error) {
	if err := service.validateDependencies(); err != nil {
		return false, err
	}

	currentIndexPath := indexFilePath(directoryPath)
	hasIndex, err := service.projectTree.Exists(currentIndexPath)
	if err != nil {
		return false, fmt.Errorf("failed to check index file %q: got error %v, expected readable filesystem", currentIndexPath, err)
	}

	return hasIndex, nil
}

func (service InitCommandService) loadChecksumSnapshot(directoryPath string) (DirectoryChecksumSnapshot, bool, error) {
	if err := service.validateDependencies(); err != nil {
		return DirectoryChecksumSnapshot{}, false, err
	}

	if repositoryWithSnapshot, ok := service.checksumRepo.(DirectoryChecksumSnapshotRepository); ok {
		return repositoryWithSnapshot.LoadSnapshot(directoryPath)
	}

	checksums, exists, err := service.checksumRepo.Load(directoryPath)
	if err != nil {
		return DirectoryChecksumSnapshot{}, false, err
	}

	files := make(map[string]FileChecksumState, len(checksums))
	for fileName, checksum := range checksums {
		files[fileName] = FileChecksumState{Checksum: checksum}
	}

	return DirectoryChecksumSnapshot{Files: files}, exists, nil
}

func (service InitCommandService) saveChecksumSnapshot(directoryPath string, snapshot DirectoryChecksumSnapshot) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}

	if repositoryWithSnapshot, ok := service.checksumRepo.(DirectoryChecksumSnapshotRepository); ok {
		return repositoryWithSnapshot.SaveSnapshot(directoryPath, snapshot)
	}

	checksums := make(map[string]string, len(snapshot.Files))
	for fileName, state := range snapshot.Files {
		checksums[fileName] = state.Checksum
	}

	return service.checksumRepo.Save(directoryPath, checksums)
}

func (service InitCommandService) computeDirectorySnapshot(fileEntries []filesystem.DirectoryEntry, stored DirectoryChecksumSnapshot) (DirectoryChecksumSnapshot, bool, map[string]struct{}, error) {
	current := newChecksumSnapshot(fileEntries)
	changed := snapshotLengthChanged(stored, fileEntries)
	changedFileNames := make(map[string]struct{})

	for _, entry := range fileEntries {
		changedEntry, err := service.updateSnapshotEntry(current.Files, entry, stored.Files, changedFileNames)
		if err != nil {
			return DirectoryChecksumSnapshot{}, false, nil, err
		}

		if changedEntry {
			changed = true
		}
	}

	if !changed && !sameSnapshotChecksums(stored.Files, current.Files) {
		changed = true
	}

	return current, changed, changedFileNames, nil
}

func newChecksumSnapshot(fileEntries []filesystem.DirectoryEntry) DirectoryChecksumSnapshot {
	return DirectoryChecksumSnapshot{Files: make(map[string]FileChecksumState, len(fileEntries))}
}

func snapshotLengthChanged(stored DirectoryChecksumSnapshot, fileEntries []filesystem.DirectoryEntry) bool {
	return len(stored.Files) != len(fileEntries)
}

func (service InitCommandService) updateSnapshotEntry(currentFiles map[string]FileChecksumState, entry filesystem.DirectoryEntry, storedFiles map[string]FileChecksumState, changedFileNames map[string]struct{}) (bool, error) {
	storedState, exists := storedFiles[entry.Name]
	if reuseStoredChecksum(entry, storedState, exists) {
		currentFiles[entry.Name] = storedState
		return false, nil
	}

	checksum, err := service.fileChecksum(entry)
	if err != nil {
		return false, err
	}

	currentState := FileChecksumState{Checksum: checksum, Size: entry.Size, ModTimeUnixNano: entry.ModTimeUnixNano}
	currentFiles[entry.Name] = currentState

	if !snapshotEntryChanged(exists, storedState, checksum) {
		return false, nil
	}

	changedFileNames[entry.Name] = struct{}{}
	return true, nil
}

func reuseStoredChecksum(entry filesystem.DirectoryEntry, storedState FileChecksumState, exists bool) bool {
	return exists && metadataUnchanged(entry, storedState) && storedState.Checksum != ""
}

func snapshotEntryChanged(exists bool, storedState FileChecksumState, checksum string) bool {
	if !exists {
		return true
	}

	return storedState.Checksum != checksum
}

func metadataUnchanged(entry filesystem.DirectoryEntry, stored FileChecksumState) bool {
	if entry.Size == 0 && entry.ModTimeUnixNano == 0 {
		return false
	}

	if stored.ModTimeUnixNano == 0 && stored.Size == 0 {
		return false
	}

	return entry.Size == stored.Size && entry.ModTimeUnixNano == stored.ModTimeUnixNano
}

func (service InitCommandService) fileChecksum(entry filesystem.DirectoryEntry) (string, error) {
	if err := service.validateDependencies(); err != nil {
		return "", err
	}

	content, err := service.fileReader.ReadFile(entry.Path)
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:]), nil
}

func sameSnapshotChecksums(stored map[string]FileChecksumState, current map[string]FileChecksumState) bool {
	if len(stored) != len(current) {
		return false
	}

	for fileName, storedState := range stored {
		currentState, exists := current[fileName]
		if !exists {
			return false
		}

		if storedState.Checksum != currentState.Checksum {
			return false
		}
	}

	return true
}

func (service InitCommandService) shouldReindexDirectory(directoryPath string, currentChecksums map[string]string) (bool, error) {
	if err := service.validateDependencies(); err != nil {
		return false, err
	}

	hasIndex, err := service.hasDirectoryIndex(directoryPath)
	if err != nil {
		return false, err
	}

	if !hasIndex {
		return true, nil
	}

	storedChecksums, exists, err := service.checksumRepo.Load(directoryPath)
	if err != nil {
		return false, err
	}

	if !exists {
		return true, nil
	}

	return !sameChecksums(storedChecksums, currentChecksums), nil
}

func (service InitCommandService) directoryChecksums(fileEntries []filesystem.DirectoryEntry) (map[string]string, error) {
	if err := service.validateDependencies(); err != nil {
		return nil, err
	}

	checksums := make(map[string]string, len(fileEntries))
	for _, entry := range fileEntries {
		content, err := service.fileReader.ReadFile(entry.Path)
		if err != nil {
			return nil, err
		}

		sum := sha256.Sum256([]byte(content))
		checksums[entry.Name] = hex.EncodeToString(sum[:])
	}

	return checksums, nil
}

func sameChecksums(stored map[string]string, current map[string]string) bool {
	if len(stored) != len(current) {
		return false
	}

	for fileName, storedChecksum := range stored {
		currentChecksum, exists := current[fileName]
		if !exists {
			return false
		}

		if storedChecksum != currentChecksum {
			return false
		}
	}

	return true
}
