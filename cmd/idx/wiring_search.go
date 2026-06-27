package main

import (
	featread "idx/internal/features/read"
	featsearch "idx/internal/features/search"
	sharedconfig "idx/internal/shared/config"
)

func buildSearchTuning(cfg sharedconfig.IdxConfig) featsearch.SearchServiceOptions {
	return featsearch.SearchServiceOptions{
		BM25K1:           cfg.BM25.K1,
		BM25B:            cfg.BM25.B,
		ProximityWeight:  cfg.BM25.ProximityWeight,
		PopularityWeight: cfg.BM25.PopularityWeight,
		MaxWorkers:       cfg.Search.MaxWorkers,
		CacheTTL:         cfg.Search.CacheTTL,
	}
}

func buildSearchDeps(d sharedDepsResult, readLog *featread.ReadLogRepository, tuning featsearch.SearchServiceOptions) featsearch.SearchCommandService {
	return featsearch.NewSearchCommandService(d.projectTree, d.writer, d.fileReader, d.indexRepo).
		WithTuning(tuning).
		WithReadLog(readLog)
}
