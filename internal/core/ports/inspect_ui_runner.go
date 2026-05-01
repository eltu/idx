package ports

import "idx/internal/core/domain"

// InspectUIRunner renders the inspect user interface for a loaded index.
// Example: err := runner.Run(index)
type InspectUIRunner interface {
	Run(index *domain.InvertedIndex) error
}
