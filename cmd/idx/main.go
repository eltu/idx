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
	initCommand := services.NewInitCommandService(projectTree, matcherFactory, writer)
	destroyCommand := services.NewDestroyCommandService(projectTree, writer)
	runner := cli.NewCommandRunner(os.Args, initCommand, destroyCommand)

	if err := runner.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
