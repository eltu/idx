package main

import (
	"fmt"
	"os"

	"idx/internal/adapters/handlers/cli"
	"idx/internal/adapters/repository"
	"idx/internal/core/services"
)

func main() {
	writer := cli.NewLineWriter(os.Stdout)
	projectTree := repository.NewOSProjectTree()
	matcherFactory := repository.NewGitIgnoreMatcherFactory()
	fileReader := repository.NewOSFileReader()
	indexer := services.NewBM25IndexService()
	indexRepo := repository.NewBinaryIndexRepository(projectTree)
	initCommand := services.NewInitCommandService(projectTree, matcherFactory, writer, fileReader, indexer, indexRepo)
	destroyCommand := services.NewDestroyCommandService(projectTree, writer)
	searchCommand := services.NewSearchCommandService(projectTree, writer, fileReader, indexRepo)
	runner := cli.NewCommandRunner(os.Args, initCommand, destroyCommand, searchCommand)

	if err := runner.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
