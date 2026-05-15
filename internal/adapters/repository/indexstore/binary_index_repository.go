package indexstore

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"

	"idx/internal/core/domain"
)

func init() {
	// Register domain types with gob for binary serialization
	gob.Register(&domain.InvertedIndex{})
	gob.Register(&domain.TermStats{})
	gob.Register(&domain.DocTermStats{})
	gob.Register(&domain.DocStats{})
}

// BinaryIndexRepository handles serialization/deserialization of BM25 indices using gob.
// Gob is a Go-specific binary format that is more efficient than JSON for memory usage.
type BinaryIndexRepository struct{}

// NewBinaryIndexRepository creates a new binary index repository.
func NewBinaryIndexRepository() *BinaryIndexRepository {
	return &BinaryIndexRepository{}
}

// SaveIndex serializes an index to binary format and writes it to disk.
func (repo *BinaryIndexRepository) SaveIndex(directoryPath string, index *domain.InvertedIndex) error {
	if repo == nil {
		return fmt.Errorf("failed to save index for directory %q: got nil repository, expected initialized BinaryIndexRepository", directoryPath)
	}

	indexPath := indexFilePath(directoryPath)
	indexDir := filepath.Dir(indexPath)

	if index == nil {
		return fmt.Errorf("failed to serialize index to %q: got nil index, expected valid index structure", indexPath)
	}

	// Create directory structure if needed
	if err := os.MkdirAll(indexDir, 0750); err != nil {
		return fmt.Errorf("failed to create index directory %q: got error %v, expected writable path", indexDir, err)
	}

	// Write to a temporary file first, then atomically replace the target.
	tempFile, err := os.CreateTemp(indexDir, "index-*.tmp") //nolint:gosec
	if err != nil {
		return fmt.Errorf("failed to create temporary index file in %q: got error %v, expected writable path", indexDir, err)
	}
	tempPath := tempFile.Name()
	defer func() { _ = os.Remove(tempPath) }()

	// Encode index to binary format using gob
	encoder := gob.NewEncoder(tempFile)
	if err := encoder.Encode(index); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("failed to serialize index to %q: got error %v, expected valid index structure", indexPath, err)
	}

	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary index file %q: got error %v, expected flushable file handle", tempPath, err)
	}

	if err := os.Rename(tempPath, indexPath); err != nil {
		return fmt.Errorf("failed to atomically replace index file %q with %q: got error %v, expected writable path", indexPath, tempPath, err)
	}

	return nil
}

// LoadIndex deserializes a binary index from disk.
func (repo *BinaryIndexRepository) LoadIndex(directoryPath string) (*domain.InvertedIndex, error) {
	if repo == nil {
		return nil, fmt.Errorf("failed to load index for directory %q: got nil repository, expected initialized BinaryIndexRepository", directoryPath)
	}

	indexPath := indexFilePath(directoryPath)

	f, err := os.Open(indexPath) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("failed to read index file %q: got error %v, expected readable file", indexPath, err)
	}
	defer func() { _ = f.Close() }()

	var index domain.InvertedIndex
	decoder := gob.NewDecoder(f)
	if err := decoder.Decode(&index); err != nil {
		return nil, fmt.Errorf("failed to parse index from %q: got error %v, expected valid binary format", indexPath, err)
	}

	return &index, nil
}
