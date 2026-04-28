package main

import (
	"fmt"
	"io"
	"os"

	"idx/internal/adapters/handlers/cli"
	"idx/internal/adapters/repository"
	"idx/internal/core/ports"
	"idx/internal/core/services/daemon"
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
	indexRepo := repository.NewBinaryIndexRepository()
	checksumRepo := repository.NewDirectoryChecksumRepository()
	daemonStateRepo := repository.NewDaemonStateRepository()
	processSpawner := &repository.OSProcessSpawner{}

	initCommand := indexing.NewInitCommandService(projectTree, matcherFactory, writer, fileReader, indexer, indexRepo, checksumRepo, daemonStateRepo)

	// Adapter que permite chamar init de um caminho específico
	initCommandAdapter := &initCommandAdapter{
		initService: initCommand,
		projectTree: projectTree,
	}

	daemonServiceImpl := daemon.NewDaemonService(daemonStateRepo, projectTree, writer, initCommandAdapter, processSpawner)
	daemonService := &daemonServiceAdapter{
		daemon: daemonServiceImpl,
	}

	destroyCommand := lifecycle.NewDestroyCommandService(projectTree, writer)
	searchCommand := search.NewSearchCommandService(projectTree, writer, fileReader, indexRepo)
	runner := cli.NewCommandRunner(arguments, initCommand, destroyCommand, searchCommand, daemonService)

	return runner.Run()
}

// initCommandAdapter adapta o InitCommandService para permitir chamada de um caminho específico.
type initCommandAdapter struct {
	initService indexing.InitCommandService
	projectTree ports.ProjectTree
}

func (a *initCommandAdapter) RunFromPath(projectPath string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}
	defer os.Chdir(cwd)

	if err := os.Chdir(projectPath); err != nil {
		return fmt.Errorf("failed to change to project directory %q: %w", projectPath, err)
	}

	return a.initService.Run()
}

// daemonServiceAdapter adapta o DaemonService para a interface daemonableCommand.
type daemonServiceAdapter struct {
	daemon *daemon.DaemonService
}

func (a *daemonServiceAdapter) Enable(projectPath string) error {
	return a.daemon.Enable(projectPath)
}

func (a *daemonServiceAdapter) Disable(projectPath string) error {
	return a.daemon.Disable(projectPath)
}

func (a *daemonServiceAdapter) Status() error {
	return a.daemon.Status()
}
