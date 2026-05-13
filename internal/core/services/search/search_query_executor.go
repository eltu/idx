package search

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"idx/internal/core/domain"
	"idx/internal/core/ports"
	"idx/internal/core/services/indexing"
)

func (service SearchCommandService) rankedResults(directories []string, terms []string, options ports.SearchOptions) ([]searchResult, error) {
	if err := service.validateDependencies(); err != nil {
		return nil, err
	}

	results, err := service.parallelDirectoryResults(directories, terms, options)
	if err != nil {
		return nil, err
	}

	sortResults(results)
	return results, nil
}

func (service SearchCommandService) parallelDirectoryResults(directories []string, terms []string, options ports.SearchOptions) ([]searchResult, error) {
	if len(directories) == 0 {
		return []searchResult{}, nil
	}

	jobs := make(chan string)
	resultsCh := make(chan []searchResult, len(directories))
	errCh := make(chan error, 1)
	workerCount := boundedSearchWorkerCount(len(directories))
	runDirectoryWorkers(service, workerCount, jobs, terms, options, resultsCh, errCh)
	for _, directoryPath := range directories {
		jobs <- directoryPath
	}
	close(jobs)

	return collectDirectoryResults(resultsCh, errCh)
}

func runDirectoryWorkers(service SearchCommandService, workerCount int, jobs <-chan string, terms []string, options ports.SearchOptions, resultsCh chan<- []searchResult, errCh chan<- error) {
	var workers sync.WaitGroup
	for worker := 0; worker < workerCount; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for directoryPath := range jobs {
				results, err := service.searchDirectoryIndex(directoryPath, terms, options)
				if err != nil {
					select {
					case errCh <- err:
					default:
					}
					return
				}

				resultsCh <- results
			}
		}()
	}

	go func() {
		workers.Wait()
		close(resultsCh)
	}()
}

func collectDirectoryResults(resultsCh <-chan []searchResult, errCh <-chan error) ([]searchResult, error) {
	results := make([]searchResult, 0)
	for directoryResults := range resultsCh {
		results = append(results, directoryResults...)
	}

	select {
	case err := <-errCh:
		return nil, err
	default:
		return results, nil
	}
}

func boundedSearchWorkerCount(directoryCount int) int {
	if directoryCount <= 1 {
		return 1
	}

	limit := runtime.NumCPU()
	if limit > maxSearchWorkers {
		limit = maxSearchWorkers
	}

	if limit < 1 {
		limit = 1
	}

	if directoryCount < limit {
		return directoryCount
	}

	return limit
}

func (service SearchCommandService) computeRankedResults(projectRoot string, terms []string, options ports.SearchOptions) ([]searchResult, error) {
	directories, err := indexing.IndexedDirectories(service.projectTree, projectRoot)
	if err != nil {
		return nil, err
	}

	return service.rankedResults(directories, terms, options)
}

func (service SearchCommandService) searchDirectoryIndex(directoryPath string, terms []string, options ports.SearchOptions) ([]searchResult, error) {
	if err := service.validateDependencies(); err != nil {
		return nil, err
	}

	index, err := service.indexRepo.LoadIndex(directoryPath)
	if err != nil {
		return nil, err
	}

	scores := scoreDocuments(index, terms, options.Operator)
	metadataMatches := metadataMatchedDocuments(index, options)
	if shouldRelaxSearch(terms, options) {
		return service.relaxedDirectoryResults(index, directoryPath, terms, options, metadataMatches)
	}

	scores = filteredScores(scores, metadataMatches, len(terms) == 0)
	normalizeScores(scores)

	return service.buildSearchResults(directoryPath, terms, options.Context, scores, 0)
}

func shouldRelaxSearch(terms []string, options ports.SearchOptions) bool {
	if !options.RelaxationEnabled {
		return false
	}

	if options.Operator != ports.SearchOperatorAND {
		return false
	}

	if len(terms) <= 1 {
		return false
	}

	return len(terms) > options.RelaxationMinExclusive
}

func (service SearchCommandService) relaxedDirectoryResults(index *domain.InvertedIndex, directoryPath string, terms []string, options ports.SearchOptions, metadataMatches map[string]struct{}) ([]searchResult, error) {
	combined := make(map[string]searchResult)
	candidates := relaxationCandidates(terms, options.RelaxationMinExclusive)
	for _, candidateTerms := range candidates {
		candidateScores := scoreDocuments(index, candidateTerms, ports.SearchOperatorAND)
		candidateScores = filteredScores(candidateScores, metadataMatches, false)
		normalizeScores(candidateScores)

		candidateResults, err := service.buildSearchResults(directoryPath, candidateTerms, options.Context, candidateScores, len(candidateTerms))
		if err != nil {
			return nil, err
		}

		mergeRelaxedResults(combined, candidateResults)
	}

	return mapResults(combined), nil
}

func relaxationCandidates(terms []string, minExclusive int) [][]string {
	candidates := make([][]string, 0, len(terms))
	for size := len(terms); size >= 1; size-- {
		candidates = append(candidates, terms[:size])
	}

	return candidates
}

func mergeRelaxedResults(combined map[string]searchResult, partial []searchResult) {
	for _, result := range partial {
		key := filepath.Join(result.directoryPath, result.fileName)
		existing, exists := combined[key]
		if !exists || relaxedResultBetter(result, existing) {
			combined[key] = result
		}
	}
}

func relaxedResultBetter(candidate searchResult, current searchResult) bool {
	if candidate.matchedTerms != current.matchedTerms {
		return candidate.matchedTerms > current.matchedTerms
	}

	return candidate.score > current.score
}

func mapResults(combined map[string]searchResult) []searchResult {
	results := make([]searchResult, 0, len(combined))
	for _, result := range combined {
		results = append(results, result)
	}

	return results
}

func (service SearchCommandService) buildSearchResults(directoryPath string, terms []string, contextSize int, scores map[string]float64, matchedTerms int) ([]searchResult, error) {
	results := make([]searchResult, 0, len(scores))
	for fileName, score := range scores {
		result, err := service.buildSearchResult(directoryPath, fileName, terms, contextSize, score, matchedTerms)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				results = append(results, searchResult{directoryPath: directoryPath, fileName: fileName, score: score, stale: true})
				continue
			}
			return nil, err
		}
		results = append(results, result)
	}
	return results, nil
}

func (service SearchCommandService) buildSearchResult(directoryPath string, fileName string, terms []string, contextSize int, score float64, matchedTerms int) (searchResult, error) {
	lines, err := service.resultMatchedLines(directoryPath, fileName, terms, contextSize)
	if err != nil {
		return searchResult{}, err
	}

	return searchResult{
		directoryPath:     directoryPath,
		fileName:          fileName,
		matchedLines:      lines,
		score:             score + fileNameMatchBonus(terms, fileName),
		matchedTerms:      matchedTerms,
		termConcentration: maxTermsOnLine(lines, terms),
	}, nil
}

func (service SearchCommandService) resultMatchedLines(directoryPath string, fileName string, terms []string, contextSize int) ([]matchedLine, error) {
	if len(terms) == 0 {
		return []matchedLine{}, nil
	}

	return service.allMatchingLines(directoryPath, fileName, terms, contextSize)
}

func (service SearchCommandService) allMatchingLines(directoryPath string, fileName string, terms []string, contextSize int) ([]matchedLine, error) {
	if err := service.validateDependencies(); err != nil {
		return nil, err
	}

	content, err := service.fileReader.ReadFile(filepath.Join(directoryPath, fileName))
	if err != nil {
		return nil, err
	}

	return matchingLinesInContent(content, terms, contextSize), nil
}

func relativeResultPath(projectRoot string, directoryPath string, documentName string) (string, error) {
	absoluteFilePath := filepath.Join(directoryPath, documentName)
	projectRelativePath, err := filepath.Rel(projectRoot, absoluteFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve project-relative path: got root %q and file %q with error %v, expected a file within project root", projectRoot, absoluteFilePath, err)
	}

	formattedPath := filepath.ToSlash(projectRelativePath)
	if formattedPath == documentName {
		return "./" + formattedPath, nil
	}

	return formattedPath, nil
}

func uniqueQueryTerms(query string) []string {
	tokens := domain.TokenizeText(query)
	unique := make(map[string]struct{})
	terms := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if _, exists := unique[token.Token]; exists {
			continue
		}

		unique[token.Token] = struct{}{}
		terms = append(terms, token.Token)
	}

	return terms
}
