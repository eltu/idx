package ports

import "idx/internal/core/domain"

// IndexRepository defines saving and loading index to storage.
type IndexRepository interface {
	SaveIndex(directoryPath string, index *domain.InvertedIndex) error
	LoadIndex(directoryPath string) (*domain.InvertedIndex, error)
}

type FileChecksumState struct {
	Checksum        string
	Size            int64
	ModTimeUnixNano int64
}

type DirectoryChecksumSnapshot struct {
	Files map[string]FileChecksumState
}

// DirectoryChecksumRepository defines checksum metadata storage per directory.
type DirectoryChecksumRepository interface {
	Load(directoryPath string) (map[string]string, bool, error)
	Save(directoryPath string, checksums map[string]string) error
}

// DirectoryChecksumSnapshotRepository provides metadata-aware checksum snapshots.
// Implementations can use this to avoid rehashing unchanged files during sync.
type DirectoryChecksumSnapshotRepository interface {
	LoadSnapshot(directoryPath string) (DirectoryChecksumSnapshot, bool, error)
	SaveSnapshot(directoryPath string, snapshot DirectoryChecksumSnapshot) error
}
