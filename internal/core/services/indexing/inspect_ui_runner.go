package indexing

import "idx/internal/core/domain"

type defaultInspectUIRunner struct{}

func (defaultInspectUIRunner) Run(index *domain.InvertedIndex) error {
	return runInspectTUI(index)
}
