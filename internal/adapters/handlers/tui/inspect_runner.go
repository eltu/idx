package tui

import (
	"idx/internal/core/domain"
	"idx/internal/core/ports"
	"idx/internal/core/services/indexing"
)

type inspectRunner struct{}

// NewInspectRunner builds the inspect UI adapter implementation.
// Example: runner := NewInspectRunner()
func NewInspectRunner() ports.InspectUIRunner {
	return inspectRunner{}
}

func (inspectRunner) Run(index *domain.InvertedIndex) error {
	return indexing.RunInspectUI(index)
}
