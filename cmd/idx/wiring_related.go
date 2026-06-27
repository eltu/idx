package main

import (
	featread "idx/internal/features/read"
	featrelated "idx/internal/features/related"
)

func buildRelatedDeps(d sharedDepsResult, coReadRepo *featread.CoReadMatrixRepository) featrelated.RelatedCommandService {
	return featrelated.NewRelatedCommandService(d.projectTree, d.indexRepo, coReadRepo, d.writer)
}
