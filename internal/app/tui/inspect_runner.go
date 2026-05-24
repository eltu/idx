package tui

import (
	"idx/internal/features/indexing"
)

type inspectRunner struct{}

// NewInspectRunner builds the inspect UI adapter implementation.
// Example: runner := NewInspectRunner()
func NewInspectRunner() indexing.InspectUIRunner {
	return inspectRunner{}
}

func (inspectRunner) Run(index *indexing.InvertedIndex) error {
	return runInspectTUI(index)
}
