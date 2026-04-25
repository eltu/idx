package ports

import "idx/internal/core/domain"

// IndexRepository defines saving and loading index to storage.
type IndexRepository interface {
	SaveIndex(directoryPath string, index *domain.InvertedIndex) error
	LoadIndex(directoryPath string) (*domain.InvertedIndex, error)
}

// DirectoryChecksumRepository defines checksum metadata storage per directory.
type DirectoryChecksumRepository interface {
	Load(directoryPath string) (map[string]string, bool, error)
	Save(directoryPath string, checksums map[string]string) error
}
