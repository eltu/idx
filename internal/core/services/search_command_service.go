package services

import (
	"fmt"
	"math"
	"path/filepath"
	"runtime"
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
}

// NewSearchCommandService builds the search use case.
// Example: service := NewSearchCommandService(projectTree, output, indexRepo)
func NewSearchCommandService(projectTree ports.ProjectTree, output ports.TextOutput, fileReader ports.FileReader, indexRepo searchableIndexRepository) SearchCommandService {
	return SearchCommandService{
		projectTree: projectTree,
		output:      output,
		fileReader:  fileReader,
		indexRepo:   indexRepo,
	}
}

func (service SearchCommandService) Run(query string) error {
	currentDir, err := service.projectTree.CurrentDir()
	if err != nil {
		return fmt.Errorf("failed to resolve current directory: got error %v, expected a readable working directory", err)
	}

	projectRoot, err := service.projectTree.FindGitRoot(currentDir)
	if err != nil {
		return err
	}

	indexedDirectories, err := service.indexedDirectories(projectRoot)
	if err != nil {
		return err
	}

	results, err := service.rankedResults(indexedDirectories, query)
	if err != nil {
		return err
	}

	if len(results) == 0 {
		return service.output.WriteLine("Nenhum resultado encontrado.")
	}

	return service.writeResults(results, projectRoot)
}

func (service SearchCommandService) writeResults(results []searchResult, projectRoot string) error {
	for _, result := range results {
		projectRelativePath, err := relativeResultPath(projectRoot, result.directoryPath, result.fileName)
		if err != nil {
			return err
		}

		header := fmt.Sprintf("%s (score: %.4f)", projectRelativePath, result.score)
		if err := service.output.WriteLine(header); err != nil {
			return err
		}

		for i, ml := range result.matchedLines {
			prefix := "├──"
			if i == len(result.matchedLines)-1 {
				prefix = "└──"
			}
			entry := fmt.Sprintf("%s %d: %s", prefix, ml.lineNumber, ml.content)
			if err := service.output.WriteLine(entry); err != nil {
				return err
			}
		}
	}

	return nil
}

func (service SearchCommandService) indexedDirectories(projectRoot string) ([]string, error) {
	directories := make([]string, 0)
	if err := service.collectIndexedDirectories(projectRoot, &directories); err != nil {
		return nil, err
	}

	return directories, nil
}

func (service SearchCommandService) collectIndexedDirectories(directoryPath string, directories *[]string) error {
	indexPath := filepath.Join(directoryPath, ".idx", "index.idx")
	hasIndex, err := service.projectTree.Exists(indexPath)
	if err != nil {
		return err
	}

	if hasIndex {
		*directories = append(*directories, directoryPath)
	}

	entries, err := service.projectTree.ReadDir(directoryPath)
	if err != nil {
		return fmt.Errorf("failed to read directory %q: got error %v, expected a readable directory", directoryPath, err)
	}

	for _, entry := range entries {
		if !entry.IsDir || entry.Name == ".git" || entry.Name == ".idx" {
			continue
		}

		if err := service.collectIndexedDirectories(entry.Path, directories); err != nil {
			return err
		}
	}

	return nil
}

func (service SearchCommandService) rankedResults(directories []string, query string) ([]searchResult, error) {
	terms := uniqueQueryTerms(query)
	results, err := service.parallelDirectoryResults(directories, terms)
	if err != nil {
		return nil, err
	}

	sortResults(results)
	return results, nil
}

func (service SearchCommandService) parallelDirectoryResults(directories []string, terms []string) ([]searchResult, error) {
	if len(directories) == 0 {
		return []searchResult{}, nil
	}

	jobs := make(chan string)
	resultsCh := make(chan []searchResult, len(directories))
	errCh := make(chan error, 1)
	workerCount := boundedSearchWorkerCount(len(directories))
	runDirectoryWorkers(service, workerCount, jobs, terms, resultsCh, errCh)
	for _, directoryPath := range directories {
		jobs <- directoryPath
	}
	close(jobs)

	return collectDirectoryResults(resultsCh, errCh)
}

func runDirectoryWorkers(service SearchCommandService, workerCount int, jobs <-chan string, terms []string, resultsCh chan<- []searchResult, errCh chan<- error) {
	var workers sync.WaitGroup
	for worker := 0; worker < workerCount; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for directoryPath := range jobs {
				results, err := service.searchDirectoryIndex(directoryPath, terms)
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

func (service SearchCommandService) searchDirectoryIndex(directoryPath string, terms []string) ([]searchResult, error) {
	index, err := service.indexRepo.LoadIndex(directoryPath)
	if err != nil {
		return nil, err
	}

	scores := scoreDocuments(index, terms)
	normalizeScores(scores)
	results := make([]searchResult, 0, len(scores))
	for fileName, score := range scores {
		lines, err := service.allMatchingLines(directoryPath, fileName, terms)
		if err != nil {
			return nil, err
		}

		results = append(results, searchResult{directoryPath: directoryPath, fileName: fileName, matchedLines: lines, score: score})
	}

	return results, nil
}

func (service SearchCommandService) allMatchingLines(directoryPath string, fileName string, terms []string) ([]matchedLine, error) {
	content, err := service.fileReader.ReadFile(filepath.Join(directoryPath, fileName))
	if err != nil {
		return nil, err
	}

	return matchingLinesInContent(content, terms), nil
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

func matchingLinesInContent(content string, terms []string) []matchedLine {
	var matches []matchedLine
	for index, line := range strings.Split(content, "\n") {
		if lineContainsAnyTerm(line, terms) {
			matches = append(matches, matchedLine{lineNumber: index + 1, content: line})
		}
	}
	return matches
}

// lineContainsAnyTerm returns true when the line contains at least one term as a whole word token.
// Whole-word is defined by the same character class as the tokenizer: [a-zA-Z0-9_].
func lineContainsAnyTerm(line string, terms []string) bool {
	lower := strings.ToLower(line)
	for _, term := range terms {
		if lineContainsTerm(lower, term) {
			return true
		}
	}
	return false
}

func lineContainsTerm(lowerLine string, term string) bool {
	start := 0
	for {
		idx := strings.Index(lowerLine[start:], term)
		if idx < 0 {
			return false
		}
		abs := start + idx
		before := abs == 0 || !isWordChar(lowerLine[abs-1])
		after := abs+len(term) == len(lowerLine) || !isWordChar(lowerLine[abs+len(term)])
		if before && after {
			return true
		}
		start = abs + 1
	}
}

func isWordChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_'
}

func sortResults(results []searchResult) {
	for i := 1; i < len(results); i++ {
		for j := i; j > 0; j-- {
			left, right := results[j-1], results[j]
			if left.score > right.score {
				break
			}
			if left.score == right.score {
				leftPath := filepath.Join(left.directoryPath, left.fileName)
				rightPath := filepath.Join(right.directoryPath, right.fileName)
				if leftPath <= rightPath {
					break
				}
			}
			results[j-1], results[j] = results[j], results[j-1]
		}
	}
}

// normalizeScores scales all scores in-place to [0, 1] using min-max.
// Each directory is an independent BM25 corpus: small directories produce
// higher IDF than large ones, making raw scores incomparable across directories.
func normalizeScores(scores map[string]float64) {
	if len(scores) == 0 {
		return
	}
	minScore, maxScore := scoreRange(scores)
	spread := maxScore - minScore
	if spread == 0 {
		for key := range scores {
			scores[key] = 1.0
		}
		return
	}
	for key, score := range scores {
		scores[key] = (score - minScore) / spread
	}
}

func scoreRange(scores map[string]float64) (float64, float64) {
	minScore := math.MaxFloat64
	maxScore := -math.MaxFloat64
	for _, score := range scores {
		if score < minScore {
			minScore = score
		}
		if score > maxScore {
			maxScore = score
		}
	}
	return minScore, maxScore
}

func scoreDocuments(index *domain.InvertedIndex, terms []string) map[string]float64 {
	matchingDocuments := documentsContainingAllTerms(index, terms)
	if len(matchingDocuments) == 0 {
		return map[string]float64{}
	}

	scores := make(map[string]float64)
	for _, term := range terms {
		termStats := index.Terms[term]
		if termStats == nil {
			continue
		}

		addTermScores(scores, index, termStats, matchingDocuments)
	}

	applyProximityBonus(scores, index, terms, matchingDocuments)

	return scores
}

func applyProximityBonus(scores map[string]float64, index *domain.InvertedIndex, terms []string, matchingDocuments map[string]struct{}) {
	for filePath := range matchingDocuments {
		scores[filePath] += proximityBonusForDocument(index, filePath, terms)
	}
}

func proximityBonusForDocument(index *domain.InvertedIndex, filePath string, terms []string) float64 {
	if len(terms) < 2 {
		return 0
	}

	totalPairScore := 0.0
	pairCount := 0
	for termIndex := 0; termIndex < len(terms)-1; termIndex++ {
		distance, ok := minimumDistanceForTermPair(index, filePath, terms[termIndex], terms[termIndex+1])
		if !ok {
			continue
		}

		totalPairScore += 1.0 / (1.0 + float64(distance))
		pairCount++
	}

	if pairCount == 0 {
		return 0
	}

	return proximityWeight * (totalPairScore / float64(pairCount))
}

func minimumDistanceForTermPair(index *domain.InvertedIndex, filePath string, leftTerm string, rightTerm string) (int, bool) {
	leftDocTerm := index.Terms[leftTerm].Docs[filePath]
	rightDocTerm := index.Terms[rightTerm].Docs[filePath]
	if len(leftDocTerm.Positions) == 0 || len(rightDocTerm.Positions) == 0 {
		return 0, false
	}

	minDistance := math.MaxInt
	for _, leftPos := range leftDocTerm.Positions {
		for _, rightPos := range rightDocTerm.Positions {
			distance := absInt(leftPos - rightPos)
			if distance < minDistance {
				minDistance = distance
			}
		}
	}

	return minDistance, true
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}

	return value
}

func documentsContainingAllTerms(index *domain.InvertedIndex, terms []string) map[string]struct{} {
	if len(terms) == 0 {
		return map[string]struct{}{}
	}

	firstTermStats := index.Terms[terms[0]]
	if firstTermStats == nil {
		return map[string]struct{}{}
	}

	matchingDocuments := make(map[string]struct{})
	for filePath := range firstTermStats.Docs {
		matchingDocuments[filePath] = struct{}{}
	}

	for _, term := range terms[1:] {
		termStats := index.Terms[term]
		if termStats == nil {
			return map[string]struct{}{}
		}

		filterDocumentsByTerm(matchingDocuments, termStats)
	}

	return matchingDocuments
}

func filterDocumentsByTerm(matchingDocuments map[string]struct{}, termStats *domain.TermStats) {
	for filePath := range matchingDocuments {
		if _, exists := termStats.Docs[filePath]; exists {
			continue
		}

		delete(matchingDocuments, filePath)
	}
}

func addTermScores(scores map[string]float64, index *domain.InvertedIndex, termStats *domain.TermStats, matchingDocuments map[string]struct{}) {
	for filePath, docTerm := range termStats.Docs {
		if _, exists := matchingDocuments[filePath]; !exists {
			continue
		}

		docStats := index.Documents[filePath]
		if docStats == nil {
			continue
		}

		scores[filePath] += domain.BM25Score(docTerm.TF, termStats.IDF, docStats.Length, index.AverageDocLength, bm25K1, bm25B)
	}
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
