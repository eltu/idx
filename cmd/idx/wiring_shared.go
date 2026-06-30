package main

import (
	"io"
	"os"

	appcli "idx/internal/app/cli"
	idxstorage "idx/internal/features/indexing/storage"
	sharedconfig "idx/internal/shared/config"
	sharedfs "idx/internal/shared/filesystem"
)

type sharedDepsResult struct {
	rawOutput      io.Writer
	writer         *appcli.LineWriter
	projectTree    sharedfs.ProjectTree
	matcherFactory sharedfs.IgnoreMatcherBuilder
	fileReader     sharedfs.FileReader
	indexRepo      *idxstorage.BinaryIndexRepository
	checksumRepo   *idxstorage.DirectoryChecksumRepository
	projectRoot    string
	cfg            sharedconfig.IdxConfig
	overrides      []string
	configFilePath string
}

func sharedDeps(output io.Writer) (sharedDepsResult, error) {
	writer := appcli.NewLineWriter(output)
	projectTree := sharedfs.NewOSProjectTree()
	matcherFactory := sharedfs.IgnoreMatcherBuilder(sharedfs.NewGitIgnoreMatcherFactory())
	fileReader := sharedfs.NewOSFileReader()
	indexRepo := idxstorage.NewBinaryIndexRepository()
	checksumRepo := idxstorage.NewDirectoryChecksumRepository()

	configRepo := sharedconfig.NewYAMLRepository()
	cwd, _ := os.Getwd()
	// IDX_PROJECT_PATH is set by OSServerSpawner to guarantee path consistency between
	// the client process (which computed the project root) and the server process (whose
	// os.Getwd() may differ due to macOS firmlink resolution under /System/Volumes/Data).
	projectRoot := os.Getenv("IDX_PROJECT_PATH")
	if projectRoot == "" {
		projectRoot = gitRootFrom(cwd)
	}
	cfg, overrides, _ := configRepo.Load(projectRoot)
	configFilePath := configRepo.FilePath(projectRoot)

	matcherFactory = sharedfs.NewCompositeIgnoreMatcherFactory(
		matcherFactory,
		sharedfs.NewGlobIgnoreMatcherFactory(cfg.Index.Ignore),
	)

	return sharedDepsResult{
		rawOutput:      output,
		writer:         writer,
		projectTree:    projectTree,
		matcherFactory: matcherFactory,
		fileReader:     fileReader,
		indexRepo:      indexRepo,
		checksumRepo:   checksumRepo,
		projectRoot:    projectRoot,
		cfg:            cfg,
		overrides:      overrides,
		configFilePath: configFilePath,
	}, nil
}
