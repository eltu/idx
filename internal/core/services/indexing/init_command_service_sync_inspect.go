package indexing

import (
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"idx/internal/core/domain"
	"idx/internal/core/ports"
)

func (service InitCommandService) Sync() error {
	if err := service.validateDependencies(); err != nil {
		return err
	}

	projectRoot, matcher, staleDirectories, eligibleDirectories, err := service.syncPlan()
	if err != nil {
		return err
	}

	if err := service.removeStaleDirectories(staleDirectories); err != nil {
		return err
	}

	if err := service.syncEligibleDirectories(eligibleDirectories, projectRoot, matcher); err != nil {
		return err
	}

	return service.output.WriteLine("✅ Project indices synchronized.")
}

func (service InitCommandService) syncPlan() (string, ports.IgnoreMatcher, []string, []string, error) {
	currentDir, err := service.projectTree.CurrentDir()
	if err != nil {
		return "", nil, nil, nil, fmt.Errorf("failed to resolve current directory: got error %v, expected a readable working directory", err)
	}

	projectRoot, err := service.projectTree.FindGitRoot(currentDir)
	if err != nil {
		return "", nil, nil, nil, err
	}

	if filepath.Clean(currentDir) != filepath.Clean(projectRoot) {
		return "", nil, nil, nil, fmt.Errorf("sync must run from project root: got current directory %q, expected root directory %q", currentDir, projectRoot)
	}

	matcher, err := service.syncMatcher(projectRoot)
	if err != nil {
		return "", nil, nil, nil, err
	}

	directories, err := IndexedDirectories(service.projectTree, projectRoot)
	if err != nil {
		return "", nil, nil, nil, err
	}

	eligibleDirectories, err := eligibleDirectories(service.projectTree, projectRoot, matcher)
	if err != nil {
		return "", nil, nil, nil, err
	}

	staleDirectories := staleIndexedDirectories(directories, eligibleDirectories)
	return projectRoot, matcher, staleDirectories, eligibleDirectories, nil
}

func (service InitCommandService) syncMatcher(projectRoot string) (ports.IgnoreMatcher, error) {
	rootIndexPath := indexFilePath(projectRoot)
	hasRootIndex, err := service.projectTree.Exists(rootIndexPath)
	if err != nil {
		return nil, err
	}

	if !hasRootIndex {
		return nil, fmt.Errorf("sync requires project root to be indexed: no index found at %q, run idx init first", rootIndexPath)
	}

	matcher, err := service.matcherFactory.New(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to load ignore rules for %q: got error %v, expected a readable .gitignore configuration", projectRoot, err)
	}

	return matcher, nil
}

func (service InitCommandService) removeStaleDirectories(staleDirectories []string) error {
	for _, directoryPath := range staleDirectories {
		if err := service.removeDirectoryIndex(directoryPath); err != nil {
			return err
		}
	}

	return nil
}

func (service InitCommandService) syncEligibleDirectories(directories []string, projectRoot string, matcher ports.IgnoreMatcher) error {
	for _, directoryPath := range directories {
		if err := service.syncDirectoryIndex(directoryPath, projectRoot, matcher); err != nil {
			return err
		}
	}

	return nil
}

func (service InitCommandService) removeDirectoryIndex(directoryPath string) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}

	indexDirectoryPath := filepath.Join(directoryPath, ".idx")
	if err := service.projectTree.RemoveAll(indexDirectoryPath); err != nil {
		return err
	}

	return nil
}

func staleIndexedDirectories(indexed []string, eligible []string) []string {
	eligibleSet := make(map[string]struct{}, len(eligible))
	for _, directoryPath := range eligible {
		eligibleSet[directoryPath] = struct{}{}
	}

	stale := make([]string, 0)
	for _, directoryPath := range indexed {
		if _, ok := eligibleSet[directoryPath]; ok {
			continue
		}

		stale = append(stale, directoryPath)
	}

	sort.Strings(stale)
	return stale
}

func (service InitCommandService) Inspect(indexPath string) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}

	currentDir, err := service.projectTree.CurrentDir()
	if err != nil {
		return fmt.Errorf("failed to resolve current directory: got error %v, expected a readable working directory", err)
	}

	trimmedIndexPath := strings.TrimSpace(indexPath)
	if trimmedIndexPath == "" {
		return service.inspectAllProjectIndices(currentDir)
	}

	targetDirectory := filepath.Clean(filepath.Join(currentDir, filepath.FromSlash(path.Clean(trimmedIndexPath))))
	targetIndexPath := indexFilePath(targetDirectory)
	hasIndex, err := service.projectTree.Exists(targetIndexPath)
	if err != nil {
		return err
	}

	if !hasIndex {
		return fmt.Errorf("no index found at %q: run idx init first", targetIndexPath)
	}

	return service.writeInspectIndex(targetDirectory)
}

func (service InitCommandService) inspectAllProjectIndices(currentDir string) error {
	projectRoot, err := service.projectTree.FindGitRoot(currentDir)
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

	return service.runInspectTUIForDirectories(directories)
}

func (service InitCommandService) runInspectTUIForDirectory(directoryPath string) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}

	index, err := service.indexRepo.LoadIndex(directoryPath)
	if err != nil {
		return err
	}

	return service.inspectUI.Run(index)
}

func (service InitCommandService) runInspectTUIForDirectories(directoryPaths []string) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}

	mergedIndex, err := service.loadMergedInspectIndex(directoryPaths)
	if err != nil {
		return err
	}

	return service.inspectUI.Run(mergedIndex)
}

func (service InitCommandService) loadMergedInspectIndex(directoryPaths []string) (*domain.InvertedIndex, error) {
	sortedDirectories := append([]string(nil), directoryPaths...)
	sort.Strings(sortedDirectories)

	indicesByDirectory := make(map[string]*domain.InvertedIndex, len(sortedDirectories))
	for _, directoryPath := range sortedDirectories {
		index, err := service.indexRepo.LoadIndex(directoryPath)
		if err != nil {
			return nil, err
		}
		indicesByDirectory[directoryPath] = index
	}

	return mergeInspectIndices(indicesByDirectory), nil
}

func mergeInspectIndices(indicesByDirectory map[string]*domain.InvertedIndex) *domain.InvertedIndex {
	merged := domain.NewInvertedIndex()
	for _, directoryPath := range sortedIndexDirectories(indicesByDirectory) {
		mergeInspectIndexDirectory(merged, directoryPath, indicesByDirectory[directoryPath])
	}

	merged.DocumentCount = len(merged.Documents)
	merged.CalculateAverageDocLen()
	merged.CalculateIDF()
	return merged
}

func sortedIndexDirectories(indicesByDirectory map[string]*domain.InvertedIndex) []string {
	directories := make([]string, 0, len(indicesByDirectory))
	for directoryPath := range indicesByDirectory {
		directories = append(directories, directoryPath)
	}

	sort.Strings(directories)
	return directories
}

func mergeInspectIndexDirectory(target *domain.InvertedIndex, directoryPath string, index *domain.InvertedIndex) {
	if index == nil {
		return
	}
	mergeInspectDocuments(target, directoryPath, index)
	mergeInspectTerms(target, directoryPath, index)
}

func mergeInspectDocuments(target *domain.InvertedIndex, directoryPath string, index *domain.InvertedIndex) {
	for documentName, stats := range index.Documents {
		if stats == nil {
			continue
		}
		documentID := inspectDocumentID(directoryPath, documentName)
		target.Documents[documentID] = &domain.DocStats{Name: stats.Name, Path: stats.Path, Length: stats.Length}
	}
}

func mergeInspectTerms(target *domain.InvertedIndex, directoryPath string, index *domain.InvertedIndex) {
	for term, termStats := range index.Terms {
		if termStats == nil {
			continue
		}
		targetTerm := target.Terms[term]
		if targetTerm == nil {
			targetTerm = &domain.TermStats{Docs: make(map[string]*domain.DocTermStats)}
			target.Terms[term] = targetTerm
		}
		for documentName, docTermStats := range termStats.Docs {
			documentID := inspectDocumentID(directoryPath, documentName)
			targetTerm.Docs[documentID] = cloneInspectDocTermStats(docTermStats)
		}
	}
}

func cloneInspectDocTermStats(docTermStats *domain.DocTermStats) *domain.DocTermStats {
	if docTermStats == nil {
		return &domain.DocTermStats{}
	}
	positions := append([]int(nil), docTermStats.Positions...)
	return &domain.DocTermStats{TF: docTermStats.TF, Positions: positions}
}

func inspectDocumentID(directoryPath string, documentName string) string {
	return directoryPath + "::" + documentName
}

func (service InitCommandService) writeInspectIndex(directoryPath string) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}

	index, err := service.indexRepo.LoadIndex(directoryPath)
	if err != nil {
		return err
	}

	encoded, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode debug index for %q: got error %v, expected valid index payload", directoryPath, err)
	}

	return service.output.WriteLine(string(encoded))
}
