package repository

import (
	"encoding/json"
	"fmt"
	"os"

	"idx/internal/core/domain"
	"idx/internal/core/ports"
)

// JSONIndexRepository handles serialization/deserialization of BM25 indices.
type JSONIndexRepository struct {
	projectTree ports.ProjectTree
}

// NewJSONIndexRepository creates a new JSON index repository.
func NewJSONIndexRepository(projectTree ports.ProjectTree) *JSONIndexRepository {
	return &JSONIndexRepository{projectTree: projectTree}
}

// SaveIndex serializes an index to JSON and writes it to disk.
func (repo *JSONIndexRepository) SaveIndex(directoryPath string, index *domain.InvertedIndex) error {
	indexPath := indexFilePath(directoryPath)

	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize index: got error %v, expected valid JSON encoding", err)
	}

	if err := repo.projectTree.WriteFile(indexPath, data); err != nil {
		return fmt.Errorf("failed to write index file %q: got error %v, expected a writable path", indexPath, err)
	}

	return nil
}

// LoadIndex deserializes a JSON index from disk.
func (repo *JSONIndexRepository) LoadIndex(directoryPath string) (*domain.InvertedIndex, error) {
	indexPath := indexFilePath(directoryPath)

	// Try to read the file
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read index file %q: got error %v, expected a readable file", indexPath, err)
	}

	var index domain.InvertedIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("failed to parse index from %q: got error %v, expected valid JSON", indexPath, err)
	}

	return &index, nil
}
