package services

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"

	"idx/internal/core/domain"
	"idx/internal/core/ports"
)

const (
	bm25K1          = 1.5
	bm25B           = 0.75
	proximityWeight = 3.00
)

type SearchCommandService struct {
	projectTree ports.ProjectTree
	output      ports.TextOutput
	indexRepo   searchableIndexRepository
}

type searchableIndexRepository interface {
	LoadIndex(directoryPath string) (*domain.InvertedIndex, error)
}

type searchResult struct {
	filePath string
	score    float64
}

// NewSearchCommandService builds the search use case.
// Example: service := NewSearchCommandService(projectTree, output, indexRepo)
func NewSearchCommandService(projectTree ports.ProjectTree, output ports.TextOutput, indexRepo searchableIndexRepository) SearchCommandService {
	return SearchCommandService{
		projectTree: projectTree,
		output:      output,
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

	index, err := service.indexRepo.LoadIndex(currentDir)
	if err != nil {
		return err
	}

	results := rankedResults(index, query)
	if len(results) == 0 {
		return service.output.WriteLine("Nenhum resultado encontrado.")
	}

	return service.writeResults(results, currentDir, projectRoot)
}

func (service SearchCommandService) writeResults(results []searchResult, currentDir string, projectRoot string) error {
	for _, result := range results {
		projectRelativePath, err := relativeResultPath(currentDir, projectRoot, result.filePath)
		if err != nil {
			return err
		}

		line := fmt.Sprintf("%s (score: %.4f)", projectRelativePath, result.score)
		if err := service.output.WriteLine(line); err != nil {
			return err
		}
	}

	return nil
}

func relativeResultPath(currentDir string, projectRoot string, documentName string) (string, error) {
	absoluteFilePath := filepath.Join(currentDir, documentName)
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

func rankedResults(index *domain.InvertedIndex, query string) []searchResult {
	scores := scoreDocuments(index, uniqueQueryTerms(query))
	results := make([]searchResult, 0, len(scores))
	for filePath, score := range scores {
		results = append(results, searchResult{filePath: filePath, score: score})
	}

	sort.Slice(results, func(left int, right int) bool {
		if results[left].score == results[right].score {
			return results[left].filePath < results[right].filePath
		}

		return results[left].score > results[right].score
	})

	return results
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
