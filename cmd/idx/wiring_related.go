package main

import (
	featrelated "idx/internal/features/related"
)

func buildRelatedDeps(d sharedDepsResult, read readDepsResult) featrelated.RelatedCommandService {
	return featrelated.NewRelatedCommandService(featrelated.RelatedDeps{
		ProjectTree: d.projectTree,
		IndexRepo:   d.indexRepo,
		CoReadRepo:  read.coReadRepo,
		Output:      d.writer,
	})
}
