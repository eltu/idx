package main

import (
	"fmt"
	"os"

	"idx/internal/adapters/handlers/cli"
	"idx/internal/adapters/repository"
	"idx/internal/core/services/indexing"
	"idx/internal/core/services/lifecycle"
	"idx/internal/core/services/search"
)

func main() {
	writer := cli.NewLineWriter(os.Stdout)
	projectTree := repository.NewOSProjectTree()
	matcherFactory := repository.NewGitIgnoreMatcherFactory()
	fileReader := repository.NewOSFileReader()
	indexer := indexing.NewBM25IndexService()
	indexRepo := repository.NewBinaryIndexRepository(projectTree)
	checksumRepo := repository.NewDirectoryChecksumRepository()
	initCommand := indexing.NewInitCommandService(projectTree, matcherFactory, writer, fileReader, indexer, indexRepo, checksumRepo)
	destroyCommand := lifecycle.NewDestroyCommandService(projectTree, writer)
	searchCommand := search.NewSearchCommandService(projectTree, writer, fileReader, indexRepo)
	runner := cli.NewCommandRunner(os.Args, initCommand, destroyCommand, searchCommand)

	if err := runner.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
