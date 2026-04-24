package services

import (
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"

	"idx/internal/core/domain"
	"idx/internal/core/ports"
)

const (
	bm25K1           = 1.5
	bm25B            = 0.75
	proximityWeight  = 3.00
	maxSearchWorkers = 4
)

type SearchCommandService struct {
	projectTree ports.ProjectTree
	output      ports.TextOutput
	fileReader  ports.FileReader
	indexRepo   searchableIndexRepository
}

type searchableIndexRepository interface {
	LoadIndex(directoryPath string) (*domain.InvertedIndex, error)
}

type searchResult struct {
	directoryPath string
	fileName      string
	matchedLines  []matchedLine
	score         float64
}

type matchedLine struct {
	lineNumber int
	content    string
	isMatch    bool
}

// NewSearchCommandService builds the search use case.
// Example: service := NewSearchCommandService(projectTree, output, indexRepo).
func NewSearchCommandService(projectTree ports.ProjectTree, output ports.TextOutput, fileReader ports.FileReader, indexRepo searchableIndexRepository) SearchCommandService {
	return SearchCommandService{
		projectTree: projectTree,
		output:      output,
		fileReader:  fileReader,
		indexRepo:   indexRepo,
	}
}

// Run executes search with default options.
// Example: err := service.Run("module idx").
func (service SearchCommandService) Run(query string) error {
	return service.RunWithOptions(query, ports.SearchOptions{})
}

// RunWithOptions executes search with explicit output and context options.
// Example: err := service.RunWithOptions("module idx", ports.SearchOptions{Format: ports.SearchOutputJSON}).
func (service SearchCommandService) RunWithOptions(query string, options ports.SearchOptions) error {
	normalizedOptions := normalizedSearchOptions(options)

	currentDir, err := service.projectTree.CurrentDir()
	if err != nil {
		return fmt.Errorf("failed to resolve current directory: got error %v, expected a readable working directory", err)
	}

	projectRoot, err := service.projectTree.FindGitRoot(currentDir)
	if err != nil {
		return err
	}

	indexedDirectories, err := indexedDirectories(service.projectTree, projectRoot)
	if err != nil {
		return err
	}

	terms := uniqueQueryTerms(query)
	results, err := service.rankedResults(indexedDirectories, terms, normalizedOptions)
	if err != nil {
		return err
	}

	results = applySearchResultOptions(results, normalizedOptions, len(terms) > 0)

	if len(results) == 0 {
		return service.writeEmptySearchResults(normalizedOptions)
	}

	return service.writeSearchResults(results, projectRoot, terms, normalizedOptions)
}

func (service SearchCommandService) writeEmptySearchResults(options ports.SearchOptions) error {
	if options.Format == ports.SearchOutputJSON {
		return service.output.WriteLine("[]")
	}

	return service.output.WriteLine("Nenhum resultado encontrado.")
}

func (service SearchCommandService) writeSearchResults(results []searchResult, projectRoot string, terms []string, options ports.SearchOptions) error {
	if options.Format == ports.SearchOutputJSON {
		return service.writeResultsJSON(results, projectRoot, options)
	}

	useANSI := true
	return service.writeResults(results, projectRoot, terms, useANSI, options)
}

func normalizedSearchOptions(options ports.SearchOptions) ports.SearchOptions {
	normalized := options
	if normalized.Format == "" {
		normalized.Format = ports.SearchOutputText
	}

	if normalized.Context < 0 {
		normalized.Context = 0
	}

	if normalized.Format != ports.SearchOutputJSON {
		normalized.PrettyJSON = false
	}

	if normalized.Limit < 0 {
		normalized.Limit = 0
	}

	normalized.FileQuery = strings.TrimSpace(normalized.FileQuery)
	normalized.PathQuery = strings.TrimSpace(normalized.PathQuery)

	return normalized
}

func applySearchResultOptions(results []searchResult, options ports.SearchOptions, hasContentTerms bool) []searchResult {
	filtered := results
	if options.FilesOnly {
		filtered = filesOnlyResults(filtered)
	} else if options.MatchesOnly && hasContentTerms {
		filtered = matchesOnlyResults(filtered)
	}

	return limitedResults(filtered, options.Limit)
}

func filesOnlyResults(results []searchResult) []searchResult {
	// Use map to deduplicate by full path, keeping highest score.
	uniqueFiles := make(map[string]searchResult)
	for _, result := range results {
		fullPath := result.directoryPath + "/" + result.fileName
		if existing, exists := uniqueFiles[fullPath]; exists {
			// Keep the result with the higher score
			if result.score > existing.score {
				uniqueFiles[fullPath] = searchResult{directoryPath: result.directoryPath, fileName: result.fileName, matchedLines: []matchedLine{}, score: result.score}
			}
		} else {
			uniqueFiles[fullPath] = searchResult{directoryPath: result.directoryPath, fileName: result.fileName, matchedLines: []matchedLine{}, score: result.score}
		}
	}

	// Convert map back to slice, maintaining order by score.
	filtered := make([]searchResult, 0, len(uniqueFiles))
	for _, result := range uniqueFiles {
		filtered = append(filtered, result)
	}

	// Sort by score descending, then by filename.
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].score != filtered[j].score {
			return filtered[i].score > filtered[j].score
		}
		iPath := filtered[i].directoryPath + "/" + filtered[i].fileName
		jPath := filtered[j].directoryPath + "/" + filtered[j].fileName
		return iPath < jPath
	})

	return filtered
}

func matchesOnlyResults(results []searchResult) []searchResult {
	filtered := make([]searchResult, 0, len(results))
	for _, result := range results {
		matchedLines := onlyMatchedLines(result.matchedLines)
		if len(matchedLines) == 0 {
			continue
		}

		filtered = append(filtered, searchResult{directoryPath: result.directoryPath, fileName: result.fileName, matchedLines: matchedLines, score: result.score})
	}

	return filtered
}

func onlyMatchedLines(lines []matchedLine) []matchedLine {
	matched := make([]matchedLine, 0, len(lines))
	for _, line := range lines {
		if !line.isMatch {
			continue
		}

		matched = append(matched, line)
	}

	return matched
}

func limitedResults(results []searchResult, limit int) []searchResult {
	if limit <= 0 || len(results) <= limit {
		return results
	}

	return results[:limit]
}

func (service SearchCommandService) writeResults(results []searchResult, projectRoot string, terms []string, useANSI bool, options ports.SearchOptions) error {
	if options.FilesOnly {
		// For --files-only, just output the paths, one per line.
		for _, result := range results {
			projectRelativePath, err := relativeResultPath(projectRoot, result.directoryPath, result.fileName)
			if err != nil {
				return err
			}

			if err := service.output.WriteLine(projectRelativePath); err != nil {
				return err
			}
		}

		return nil
	}

	// Standard output with matches and context.
	for _, result := range results {
		projectRelativePath, err := relativeResultPath(projectRoot, result.directoryPath, result.fileName)
		if err != nil {
			return err
		}

		header := fmt.Sprintf("%s (score: %.4f)", coloredFilePath(projectRelativePath, useANSI), result.score)
		if err := service.output.WriteLine(header); err != nil {
			return err
		}

		for i, ml := range result.matchedLines {
			prefix := "├──"
			if i == len(result.matchedLines)-1 {
				prefix = "└──"
			}

			lineContent := ml.content
			if ml.isMatch {
				lineContent = highlightTermsInLine(ml.content, terms, useANSI)
			}

			entry := fmt.Sprintf("%s %s: %s", prefix, coloredLineNumber(ml.lineNumber, useANSI), lineContent)
			if err := service.output.WriteLine(entry); err != nil {
				return err
			}
		}

		if err := service.output.WriteLine(""); err != nil {
			return err
		}
	}

	return nil
}

func (service SearchCommandService) rankedResults(directories []string, terms []string, options ports.SearchOptions) ([]searchResult, error) {
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

func (service SearchCommandService) searchDirectoryIndex(directoryPath string, terms []string, options ports.SearchOptions) ([]searchResult, error) {
	index, err := service.indexRepo.LoadIndex(directoryPath)
	if err != nil {
		return nil, err
	}

	scores := scoreDocuments(index, terms)
	metadataMatches := metadataMatchedDocuments(index, options)
	scores = filteredScores(scores, metadataMatches, len(terms) == 0)
	normalizeScores(scores)
	results := make([]searchResult, 0, len(scores))
	for fileName, score := range scores {
		lines := []matchedLine{}
		if len(terms) > 0 {
			lines, err = service.allMatchingLines(directoryPath, fileName, terms, options.Context)
			if err != nil {
				return nil, err
			}
		}

		results = append(results, searchResult{directoryPath: directoryPath, fileName: fileName, matchedLines: lines, score: score})
	}

	return results, nil
}

func (service SearchCommandService) allMatchingLines(directoryPath string, fileName string, terms []string, contextSize int) ([]matchedLine, error) {
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
