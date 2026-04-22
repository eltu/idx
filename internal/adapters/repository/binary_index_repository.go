package repository

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"

	"idx/internal/core/domain"
	"idx/internal/core/ports"
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
type BinaryIndexRepository struct {
	projectTree ports.ProjectTree
}

// NewBinaryIndexRepository creates a new binary index repository.
func NewBinaryIndexRepository(projectTree ports.ProjectTree) *BinaryIndexRepository {
	return &BinaryIndexRepository{projectTree: projectTree}
}

// SaveIndex serializes an index to binary format and writes it to disk.
func (repo *BinaryIndexRepository) SaveIndex(directoryPath string, index *domain.InvertedIndex) error {
	indexPath := indexFilePath(directoryPath)
	indexDir := filepath.Dir(indexPath)

	// Create directory structure if needed
	if err := os.MkdirAll(indexDir, 0755); err != nil {
		return fmt.Errorf("failed to create index directory %q: got error %v, expected writable path", indexDir, err)
	}

	// Create file for binary writing
	f, err := os.Create(indexPath)
	if err != nil {
		return fmt.Errorf("failed to create index file %q: got error %v, expected writable path", indexPath, err)
	}
	defer f.Close()

	// Encode index to binary format using gob
	encoder := gob.NewEncoder(f)
	if err := encoder.Encode(index); err != nil {
		return fmt.Errorf("failed to serialize index to %q: got error %v, expected valid index structure", indexPath, err)
	}

	return nil
}

// LoadIndex deserializes a binary index from disk.
func (repo *BinaryIndexRepository) LoadIndex(directoryPath string) (*domain.InvertedIndex, error) {
	indexPath := indexFilePath(directoryPath)

	f, err := os.Open(indexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read index file %q: got error %v, expected readable file", indexPath, err)
	}
	defer f.Close()

	var index domain.InvertedIndex
	decoder := gob.NewDecoder(f)
	if err := decoder.Decode(&index); err != nil {
		return nil, fmt.Errorf("failed to parse index from %q: got error %v, expected valid binary format", indexPath, err)
	}

	return &index, nil
}
