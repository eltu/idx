package main

import (
	featread "idx/internal/features/read"
)

type readDepsResult struct {
	readLog     *featread.ReadLogRepository
	coReadRepo  *featread.CoReadMatrixRepository
	readService featread.ReadCommandService
}

func buildReadDeps(d sharedDepsResult) readDepsResult {
	readLog := featread.NewReadLogRepository()
	coReadRepo := featread.NewCoReadMatrixRepository()
	fileStreamer := featread.NewOSFileStreamer()
	readSvc := featread.NewReadCommandService(d.projectTree, fileStreamer, d.writer).
		WithReadLog(readLog)
	return readDepsResult{readLog: readLog, coReadRepo: coReadRepo, readService: readSvc}
}
