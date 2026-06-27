package main

import (
	appserver "idx/internal/app/server"
	featsearch "idx/internal/features/search"
	idxipc "idx/internal/shared/ipc"
)

func buildIndexServer(d sharedDepsResult, idx indexingDepsResult, read readDepsResult, tuning featsearch.SearchServiceOptions) appserver.ServerRunner {
	socketPath := idxipc.SocketPath(d.projectRoot)
	return appserver.NewServer(appserver.ServerDeps{
		ProjectTree:     d.projectTree,
		MatcherFactory:  d.matcherFactory,
		FileReader:      d.fileReader,
		Indexer:         idx.indexer,
		IndexRepo:       d.indexRepo,
		ChecksumRepo:    d.checksumRepo,
		DaemonRepo:      idx.serverDaemon,
		ReadLogRepo:     read.readLog,
		CoReadRepo:      read.coReadRepo,
		SearchTuning:    tuning,
		SocketPath:      socketPath,
		ProjectRoot:     d.projectRoot,
		Config:          d.cfg,
		ConfigFilePath:  d.configFilePath,
		ConfigOverrides: d.overrides,
	})
}
