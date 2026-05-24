package search

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"idx/internal/features/indexing"
	"idx/internal/features/read"
	"idx/internal/shared/filesystem"
	"idx/internal/shared/output"
)

// SearchServiceOptions holds tuning parameters for the search service.
// Zero values are replaced by DefaultIdxConfig() equivalents in WithTuning.
type SearchServiceOptions struct {
	BM25K1           float64
	BM25B            float64
	ProximityWeight  float64
	PopularityWeight float64
	MaxWorkers       int
	CacheTTL         time.Duration
}

type searchTuning struct {
	bm25K1           float64
	bm25B            float64
	proximityWeight  float64
	popularityWeight float64
	maxWorkers       int
	cacheTTL         time.Duration
}

func defaultSearchTuning() searchTuning {
	return searchTuning{
		bm25K1:           1.5,
		bm25B:            0.75,
		proximityWeight:  3.00,
		popularityWeight: 0.3,
		maxWorkers:       4,
		cacheTTL:         time.Minute,
	}
}

type cacheEntry struct {
	results   []searchResult
	expiresAt time.Time
}

type searchCache struct {
	mu      sync.Mutex
	entries map[string]cacheEntry
	ttl     time.Duration
}

type SearchCommandService struct {
	projectTree  filesystem.ProjectTree
	output       output.Writer
	fileReader   filesystem.FileReader
	indexRepo    IndexLoader
	readLogRepo  read.LogRepository
	cache        *searchCache
	cacheEnabled bool
	tuning       searchTuning
}

// IndexLoader loads an inverted index from a directory path.
type IndexLoader interface {
	LoadIndex(directoryPath string) (*indexing.InvertedIndex, error)
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

// NewSearchCommandService builds the search use case with built-in defaults.
// Use WithTuning to override BM25 parameters or cache settings from .idx.yml.
// Example: service := NewSearchCommandService(projectTree, output, fileReader, indexRepo).
func NewSearchCommandService(projectTree filesystem.ProjectTree, output output.Writer, fileReader filesystem.FileReader, indexRepo IndexLoader) SearchCommandService {
	tuning := defaultSearchTuning()
	return SearchCommandService{
		projectTree:  projectTree,
		output:       output,
		fileReader:   fileReader,
		indexRepo:    indexRepo,
		cache:        &searchCache{entries: make(map[string]cacheEntry), ttl: tuning.cacheTTL},
		cacheEnabled: true,
		tuning:       tuning,
	}
}

// WithTuning applies BM25 and cache parameters from the project config.
// Non-zero fields in opts replace the corresponding default.
// Example: service = service.WithTuning(SearchServiceOptions{BM25K1: 1.2, MaxWorkers: 8}).
func (service SearchCommandService) WithTuning(opts SearchServiceOptions) SearchCommandService {
	if opts.BM25K1 > 0 {
		service.tuning.bm25K1 = opts.BM25K1
	}
	if opts.BM25B > 0 {
		service.tuning.bm25B = opts.BM25B
	}
	if opts.ProximityWeight > 0 {
		service.tuning.proximityWeight = opts.ProximityWeight
	}
	if opts.PopularityWeight > 0 {
		service.tuning.popularityWeight = opts.PopularityWeight
	}
	if opts.MaxWorkers > 0 {
		service.tuning.maxWorkers = opts.MaxWorkers
	}
	if opts.CacheTTL > 0 {
		service.tuning.cacheTTL = opts.CacheTTL
		if service.cache != nil {
			service.cache.ttl = opts.CacheTTL
		}
	}
	return service
}

// WithReadLog attaches a read log repository so search can apply a popularity boost.
// When not set, the popularity weight in SearchOptions is still applied but every
// file gets a boost of zero (no read history available).
// Example: service = service.WithReadLog(readLogRepo).
func (service SearchCommandService) WithReadLog(repo read.LogRepository) SearchCommandService {
	service.readLogRepo = repo
	return service
}

// loadPopularityMap loads all read log entries and indexes them by absolute file path
// so buildSearchResult can do O(1) lookups by filepath.Join(directoryPath, fileName).
// Returns nil when no repository is wired or on load error (silently degrades).
func (service SearchCommandService) loadPopularityMap(projectRoot string) map[string]read.LogEntry {
	if service.readLogRepo == nil {
		return nil
	}
	entries, err := service.readLogRepo.LoadAll(projectRoot)
	if err != nil {
		return nil
	}
	m := make(map[string]read.LogEntry, len(entries))
	for _, e := range entries {
		m[filepath.Join(projectRoot, e.Path)] = e
	}
	return m
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
	return service.RunWithOptions(query, Options{PopularityWeight: service.tuning.popularityWeight})
}

// RunWithOptions executes search with explicit output and context options.
// Example: err := service.RunWithOptions("module idx", Options{Format: OutputJSON}).
func (service SearchCommandService) RunWithOptions(query string, options Options) error {
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

func (service SearchCommandService) runRankedSearch(query string, projectRoot string, terms []string, options Options) ([]searchResult, error) {
	if service.cacheEnabled && service.cache != nil {
		return service.cachedRankedResults(query, projectRoot, terms, options)
	}

	return service.computeRankedResults(projectRoot, terms, options)
}

func (service SearchCommandService) cachedRankedResults(query string, projectRoot string, terms []string, options Options) ([]searchResult, error) {
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
		return fmt.Errorf("failed to run search: got nil projectTree dependency, expected non-nil filesystem.ProjectTree")
	}

	if service.output == nil {
		return fmt.Errorf("failed to run search: got nil output dependency, expected non-nil output.Writer")
	}

	if service.fileReader == nil {
		return fmt.Errorf("failed to run search: got nil fileReader dependency, expected non-nil filesystem.FileReader")
	}

	if service.indexRepo == nil {
		return fmt.Errorf("failed to run search: got nil index repository dependency, expected non-nil IndexLoader")
	}

	return nil
}
