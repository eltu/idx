package search

import (
	"fmt"
	"sync"
	"time"

	"idx/internal/core/domain"
	"idx/internal/core/ports"
)

const (
	bm25K1           = 1.5
	bm25B            = 0.75
	proximityWeight  = 3.00
	maxSearchWorkers = 4
)

const (
	searchCacheTTL = 1 * time.Minute
)

type cacheEntry struct {
	results   []searchResult
	expiresAt time.Time
}

type searchCache struct {
	mu      sync.Mutex
	entries map[string]cacheEntry
}

type SearchCommandService struct {
	projectTree  ports.ProjectTree
	output       ports.TextOutput
	fileReader   ports.FileReader
	indexRepo    searchableIndexRepository
	cache        *searchCache
	cacheEnabled bool
}

type searchableIndexRepository interface {
	LoadIndex(directoryPath string) (*domain.InvertedIndex, error)
}

type searchResult struct {
	directoryPath string
	fileName      string
	matchedLines  []matchedLine
	score         float64
	matchedTerms  int
	// termConcentration is the maximum number of distinct query terms that
	// co-occur on a single matched line. Used as a tiebreaker when BM25 scores
	// are equal: a line containing all terms (e.g. "err := root.Execute()")
	// ranks above files where the same terms appear on separate lines.
	termConcentration int
	// stale marks a result whose file was present in the index but no longer
	// exists on disk. Shown with a warning instead of matched lines.
	stale bool
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
		projectTree:  projectTree,
		output:       output,
		fileReader:   fileReader,
		indexRepo:    indexRepo,
		cache:        &searchCache{entries: make(map[string]cacheEntry)},
		cacheEnabled: true,
	}
}

// SetCacheEnabled toggles the search cache. Useful for deterministic unit tests.
func (service *SearchCommandService) SetCacheEnabled(enabled bool) {
	if service == nil {
		return
	}

	service.cacheEnabled = enabled
}

// Run executes search with default options.
// Example: err := service.Run("module idx").
func (service SearchCommandService) Run(query string) error {
	return service.RunWithOptions(query, ports.SearchOptions{})
}

// RunWithOptions executes search with explicit output and context options.
// Example: err := service.RunWithOptions("module idx", ports.SearchOptions{Format: ports.SearchOutputJSON}).
func (service SearchCommandService) RunWithOptions(query string, options ports.SearchOptions) error {
	if err := service.validateDependencies(); err != nil {
		return err
	}

	normalizedOptions := normalizedSearchOptions(options)
	projectRoot, err := service.projectRoot()
	if err != nil {
		return err
	}

	terms := uniqueQueryTerms(query)
	results, err := service.runRankedSearch(query, projectRoot, terms, normalizedOptions)
	if err != nil {
		return err
	}

	totalMatches := len(results)
	results = applySearchResultOptions(results, normalizedOptions, len(terms) > 0)

	if len(results) == 0 {
		return service.writeEmptySearchResults(normalizedOptions)
	}

	return service.writeSearchResults(results, projectRoot, terms, normalizedOptions, totalMatches)
}

func (service SearchCommandService) projectRoot() (string, error) {
	currentDir, err := service.projectTree.CurrentDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve current directory: got error %v, expected a readable working directory", err)
	}

	projectRoot, err := service.projectTree.FindGitRoot(currentDir)
	if err != nil {
		return "", err
	}

	return projectRoot, nil
}

func (service SearchCommandService) runRankedSearch(query string, projectRoot string, terms []string, options ports.SearchOptions) ([]searchResult, error) {
	if service.cacheEnabled && service.cache != nil {
		return service.cachedRankedResults(query, projectRoot, terms, options)
	}

	return service.computeRankedResults(projectRoot, terms, options)
}

func (service SearchCommandService) cachedRankedResults(query string, projectRoot string, terms []string, options ports.SearchOptions) ([]searchResult, error) {
	cacheKey := cacheKeyFor(query, options)
	results, exists := service.cache.getFromCache(cacheKey)
	if exists {
		service.cache.renewCacheTTL(cacheKey)
		return results, nil
	}

	results, err := service.computeRankedResults(projectRoot, terms, options)
	if err != nil {
		return nil, err
	}

	service.cache.setInCache(cacheKey, results)
	return results, nil
}


func (service SearchCommandService) validateDependencies() error {
	if service.projectTree == nil {
		return fmt.Errorf("failed to run search: got nil projectTree dependency, expected non-nil ports.ProjectTree")
	}

	if service.output == nil {
		return fmt.Errorf("failed to run search: got nil output dependency, expected non-nil ports.TextOutput")
	}

	if service.fileReader == nil {
		return fmt.Errorf("failed to run search: got nil fileReader dependency, expected non-nil ports.FileReader")
	}

	if service.indexRepo == nil {
		return fmt.Errorf("failed to run search: got nil index repository dependency, expected non-nil searchableIndexRepository")
	}

	return nil
}

