package main

import (
	"fmt"
	"io"
	"os"

	"idx/internal/adapters/handlers/cli"
	"idx/internal/adapters/repository"
	"idx/internal/core/services/indexing"
	"idx/internal/core/services/lifecycle"
	"idx/internal/core/services/search"
)

var exitProcess = os.Exit

func main() {
	if err := run(os.Args, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		exitProcess(1)
	}
}

func run(arguments []string, output io.Writer) error {
	writer := cli.NewLineWriter(output)
	projectTree := repository.NewOSProjectTree()
	matcherFactory := repository.NewGitIgnoreMatcherFactory()
	fileReader := repository.NewOSFileReader()
	indexer := indexing.NewBM25IndexService()
	indexRepo := repository.NewBinaryIndexRepository(projectTree)
	checksumRepo := repository.NewDirectoryChecksumRepository()
	initCommand := indexing.NewInitCommandService(projectTree, matcherFactory, writer, fileReader, indexer, indexRepo, checksumRepo)
	destroyCommand := lifecycle.NewDestroyCommandService(projectTree, writer)
	searchCommand := search.NewSearchCommandService(projectTree, writer, fileReader, indexRepo)
	runner := cli.NewCommandRunner(arguments, initCommand, destroyCommand, searchCommand)

	return runner.Run()
}
