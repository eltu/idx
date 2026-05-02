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

	eligible, err := eligibleDirectories(service.projectTree, projectRoot, matcher)
	if err != nil {
		return err
	}

	eligibleWithFiles, err := service.filterDirectoriesWithFiles(eligible, projectRoot, matcher)
	if err != nil {
		return err
	}

	missing := missingIndexDirectories(directories, eligibleWithFiles)
	if len(missing) > 0 {
		return fmt.Errorf("unindexed directories found: %v — run idx sync to update", missing)
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

// missingIndexDirectories returns eligible directories that have no index yet.
func missingIndexDirectories(indexed []string, eligible []string) []string {
	indexedSet := make(map[string]struct{}, len(indexed))
	for _, d := range indexed {
		indexedSet[d] = struct{}{}
	}

	missing := make([]string, 0)
	for _, d := range eligible {
		if _, ok := indexedSet[d]; !ok {
			missing = append(missing, d)
		}
	}

	return missing
}

// filterDirectoriesWithFiles returns only the directories that have at least one indexable file.
func (service InitCommandService) filterDirectoriesWithFiles(directories []string, projectRoot string, matcher ports.IgnoreMatcher) ([]string, error) {
	result := make([]string, 0, len(directories))
	for _, directoryPath := range directories {
		fileEntries, err := service.indexableFileEntries(directoryPath, projectRoot, matcher)
		if err != nil {
			return nil, err
		}

		if len(fileEntries) > 0 {
			result = append(result, directoryPath)
		}
	}

	return result, nil
}
