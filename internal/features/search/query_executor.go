package search

import (
	"idx/internal/features/read"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"idx/internal/features/indexing"
	
	
)

// popularityContext bundles the popularity-boost inputs used throughout result scoring.
type popularityContext struct {
	weight  float64
	entries map[string]read.LogEntry
	now     time.Time
}

// searchWorkerOutput groups the output channels shared by directory-search workers.
type searchWorkerOutput struct {
	resultsCh chan<- []searchResult
	errCh     chan<- error
}

func (service SearchCommandService) rankedResults(directories []string, terms []string, options Options, popularityMap map[string]read.LogEntry, now time.Time) ([]searchResult, error) {
	if err := service.validateDependencies(); err != nil {
		return nil, err
	}

	results, err := service.parallelDirectoryResults(directories, terms, options, popularityMap, now)
	if err != nil {
		return nil, err
	}

	sortResults(results)
	return results, nil
}

func (service SearchCommandService) parallelDirectoryResults(directories []string, terms []string, options Options, popularityMap map[string]read.LogEntry, now time.Time) ([]searchResult, error) {
	if len(directories) == 0 {
		return []searchResult{}, nil
	}

	jobs := make(chan string)
	resultsCh := make(chan []searchResult, len(directories))
	errCh := make(chan error, 1)
	workerCount := boundedSearchWorkerCount(len(directories), service.tuning.maxWorkers)
	out := searchWorkerOutput{resultsCh: resultsCh, errCh: errCh}
	service.runDirectoryWorkers(workerCount, jobs, terms, options, out, popularityMap, now)
	for _, directoryPath := range directories {
		jobs <- directoryPath
	}
	close(jobs)

	return collectDirectoryResults(resultsCh, errCh)
}

func (service SearchCommandService) runDirectoryWorkers(workerCount int, jobs <-chan string, terms []string, options Options, out searchWorkerOutput, popularityMap map[string]read.LogEntry, now time.Time) {
	var workers sync.WaitGroup
	for worker := 0; worker < workerCount; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for directoryPath := range jobs {
				results, err := service.searchDirectoryIndex(directoryPath, terms, options, popularityMap, now)
				if err != nil {
					select {
					case out.errCh <- err:
					default:
					}
					return
				}

				out.resultsCh <- results
			}
		}()
	}

	go func() {
		workers.Wait()
		close(out.resultsCh)
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

func boundedSearchWorkerCount(directoryCount, maxWorkers int) int {
	if directoryCount <= 1 {
		return 1
	}

	limit := runtime.NumCPU()
	if limit > maxWorkers {
		limit = maxWorkers
	}

	if limit < 1 {
		limit = 1
	}

	if directoryCount < limit {
		return directoryCount
	}

	return limit
}

func (service SearchCommandService) computeRankedResults(projectRoot string, terms []string, options Options) ([]searchResult, error) {
	directories, err := indexing.IndexedDirectories(service.projectTree, projectRoot)
	if err != nil {
		return nil, err
	}

	popularityMap := service.loadPopularityMap(projectRoot)
	now := time.Now()
	return service.rankedResults(directories, terms, options, popularityMap, now)
}

func (service SearchCommandService) searchDirectoryIndex(directoryPath string, terms []string, options Options, popularityMap map[string]read.LogEntry, now time.Time) ([]searchResult, error) {
	if err := service.validateDependencies(); err != nil {
		return nil, err
	}

	index, err := service.indexRepo.LoadIndex(directoryPath)
	if err != nil {
		return nil, err
	}

	scores := scoreDocuments(index, terms, options.Operator, service.tuning)
	metadataMatches := metadataMatchedDocuments(index, options)
	if shouldRelaxSearch(terms, options) {
		return service.relaxedDirectoryResults(index, directoryPath, terms, options, metadataMatches, popularityMap, now)
	}

	scores = filteredScores(scores, metadataMatches, len(terms) == 0)
	normalizeScores(scores)

	pop := popularityContext{weight: options.PopularityWeight, entries: popularityMap, now: now}
	return service.buildSearchResults(directoryPath, terms, options.Context, scores, 0, pop)
}

func shouldRelaxSearch(terms []string, options Options) bool {
	if !options.RelaxationEnabled {
		return false
	}

	if options.Operator != OperatorAND {
		return false
	}

	if len(terms) <= 1 {
		return false
	}

	return len(terms) > options.RelaxationMinExclusive
}

func (service SearchCommandService) relaxedDirectoryResults(index *indexing.InvertedIndex, directoryPath string, terms []string, options Options, metadataMatches map[string]struct{}, popularityMap map[string]read.LogEntry, now time.Time) ([]searchResult, error) {
	combined := make(map[string]searchResult)
	candidates := relaxationCandidates(terms, options.RelaxationMinExclusive)
	for _, candidateTerms := range candidates {
		candidateScores := scoreDocuments(index, candidateTerms, OperatorAND, service.tuning)
		candidateScores = filteredScores(candidateScores, metadataMatches, false)
		normalizeScores(candidateScores)

		pop := popularityContext{weight: options.PopularityWeight, entries: popularityMap, now: now}
		candidateResults, err := service.buildSearchResults(directoryPath, candidateTerms, options.Context, candidateScores, len(candidateTerms), pop)
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

func relaxedResultBetter(candidate, current searchResult) bool {
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

func (service SearchCommandService) buildSearchResults(directoryPath string, terms []string, contextSize int, scores map[string]float64, matchedTerms int, pop popularityContext) ([]searchResult, error) {
	results := make([]searchResult, 0, len(scores))
	for fileName, score := range scores {
		result, err := service.buildSearchResult(directoryPath, fileName, terms, contextSize, score, matchedTerms, pop)
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

func (service SearchCommandService) buildSearchResult(directoryPath string, fileName string, terms []string, contextSize int, score float64, matchedTerms int, pop popularityContext) (searchResult, error) {
	lines, err := service.resultMatchedLines(directoryPath, fileName, terms, contextSize)
	if err != nil {
		return searchResult{}, err
	}

	entry := pop.entries[filepath.Join(directoryPath, fileName)]
	return searchResult{
		directoryPath:     directoryPath,
		fileName:          fileName,
		matchedLines:      lines,
		score:             score + fileNameMatchBonus(terms, fileName) + popularityBonus(entry, pop.now, pop.weight),
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

func relativeResultPath(projectRoot, directoryPath, documentName string) (string, error) {
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
	tokens := indexing.TokenizeText(query)
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
