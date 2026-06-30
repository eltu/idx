package main

import (
	appserver "idx/internal/app/server"
	featsearch "idx/internal/features/search"
	idxipc "idx/internal/shared/ipc"
)

// serverWiring accumulates the results of all wiring builders so each builder
// receives a single struct instead of individual extracted fields.
type serverWiring struct {
	shared   sharedDepsResult
	indexing indexingDepsResult
	read     readDepsResult
	tuning   featsearch.SearchServiceOptions
}

func buildIndexServer(w serverWiring) appserver.ServerRunner {
	socketPath := idxipc.SocketPath(w.shared.projectRoot)
	return appserver.NewServer(appserver.ServerDeps{
		ProjectTree:     w.shared.projectTree,
		MatcherFactory:  w.shared.matcherFactory,
		FileReader:      w.shared.fileReader,
		Indexer:         w.indexing.indexer,
		IndexRepo:       w.shared.indexRepo,
		ChecksumRepo:    w.shared.checksumRepo,
		DaemonRepo:      w.indexing.serverDaemon,
		ReadLogRepo:     w.read.readLog,
		CoReadRepo:      w.read.coReadRepo,
		SearchTuning:    w.tuning,
		SocketPath:      socketPath,
		ProjectRoot:     w.shared.projectRoot,
		Config:          w.shared.cfg,
		ConfigFilePath:  w.shared.configFilePath,
		ConfigOverrides: w.shared.overrides,
	})
}
