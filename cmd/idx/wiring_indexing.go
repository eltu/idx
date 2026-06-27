package main

import (
	idxtui "idx/internal/app/tui"
	featdaemon "idx/internal/features/daemon"
	featindexing "idx/internal/features/indexing"
	featlifecycle "idx/internal/features/lifecycle"
)

type indexingDepsResult struct {
	indexer        featindexing.BM25IndexService
	initCommand    featindexing.InitCommandService
	destroyCommand featlifecycle.DestroyCommandService
	serverDaemon   *featdaemon.ServerDaemonService
	progressRunner *idxtui.InitProgressRunner
}

func buildIndexingDeps(d sharedDepsResult) indexingDepsResult {
	indexer := featindexing.NewBM25IndexService()
	inspectRunner := idxtui.NewInspectRunner()
	progressRunner := idxtui.NewInitProgressRunner()
	serverDaemon := featdaemon.NewServerDaemonService(
		featdaemon.NewServerStateRepository(), nil, d.writer,
	)
	initCmd := featindexing.NewInitCommandServiceWithProgress(featindexing.InitCommandServiceDeps{
		ProjectTree:    d.projectTree,
		MatcherFactory: d.matcherFactory,
		Output:         d.writer,
		FileReader:     d.fileReader,
		Indexer:        indexer,
		IndexRepo:      d.indexRepo,
		ChecksumRepo:   d.checksumRepo,
		DaemonRepo:     serverDaemon,
	}, inspectRunner, progressRunner)
	destroyCmd := featlifecycle.NewDestroyCommandService(d.projectTree, d.writer)
	return indexingDepsResult{
		indexer:        indexer,
		initCommand:    initCmd,
		destroyCommand: destroyCmd,
		serverDaemon:   serverDaemon,
		progressRunner: progressRunner,
	}
}
