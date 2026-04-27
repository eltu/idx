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
	if repo == nil {
		return fmt.Errorf("failed to save index for directory %q: got nil repository, expected initialized JSONIndexRepository", directoryPath)
	}

	indexPath := indexFilePath(directoryPath)

	if index == nil {
		return fmt.Errorf("failed to serialize index to %q: got nil index, expected valid index structure", indexPath)
	}

	if repo.projectTree == nil {
		return fmt.Errorf("failed to write index file %q: got nil projectTree dependency, expected non-nil ports.ProjectTree", indexPath)
	}

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
	if repo == nil {
		return nil, fmt.Errorf("failed to load index for directory %q: got nil repository, expected initialized JSONIndexRepository", directoryPath)
	}

	indexPath := indexFilePath(directoryPath)

	// Try to read the file
	data, err := os.ReadFile(indexPath) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("failed to read index file %q: got error %v, expected a readable file", indexPath, err)
	}

	var index domain.InvertedIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("failed to parse index from %q: got error %v, expected valid JSON", indexPath, err)
	}

	return &index, nil
}
