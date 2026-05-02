package indexing

import (
	"fmt"

	"idx/internal/core/ports"
)

func (service InitCommandService) Status() error {
	if err := service.validateDependencies(); err != nil {
		return err
	}

	projectRoot, matcher, err := service.statusMatcher()
	if err != nil {
		return err
	}

	directories, err := IndexedDirectories(service.projectTree, projectRoot)
	if err != nil {
		return err
	}

	if len(directories) == 0 {
		return fmt.Errorf("no index found under project root %q: run idx init first", projectRoot)
	}

	for _, directoryPath := range directories {
		if err := service.verifyDirectoryIndexCurrent(directoryPath, projectRoot, matcher); err != nil {
			return err
		}
	}

	return service.output.WriteLine("✅ Indices are up to date.")
}

func (service InitCommandService) statusMatcher() (string, ports.IgnoreMatcher, error) {
	currentDir, err := service.projectTree.CurrentDir()
	if err != nil {
		return "", nil, fmt.Errorf("failed to resolve current directory: got error %v, expected a readable working directory", err)
	}

	projectRoot, err := service.projectTree.FindGitRoot(currentDir)
	if err != nil {
		return "", nil, err
	}

	matcher, err := service.matcherFactory.New(projectRoot)
	if err != nil {
		return "", nil, fmt.Errorf("failed to load ignore rules for %q: got error %v, expected a readable .gitignore configuration", projectRoot, err)
	}

	return projectRoot, matcher, nil
}

func (service InitCommandService) verifyDirectoryIndexCurrent(directoryPath, projectRoot string, matcher ports.IgnoreMatcher) error {
	fileEntries, err := service.indexableFileEntries(directoryPath, projectRoot, matcher)
	if err != nil {
		return err
	}

	_, _, shouldReindex, err := service.reindexState(directoryPath, fileEntries)
	if err != nil {
		return err
	}

	if shouldReindex {
		return fmt.Errorf("stale index at %q: run idx sync to update", directoryPath)
	}

	return nil
}
