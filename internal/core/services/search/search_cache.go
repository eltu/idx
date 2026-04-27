package search

import (
	"time"
)

// initCache initializes a new search cache.
func initCache() searchCache {
	return searchCache{
		entries: make(map[string]cacheEntry),
	}
}

// getFromCache retrieves cached results if valid (not expired).
func (sc *searchCache) getFromCache(key string) ([]searchResult, bool) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	entry, exists := sc.entries[key]
	if !exists {
		return nil, false
	}

	// Check if expired.
	if time.Now().After(entry.expiresAt) {
		delete(sc.entries, key)
		return nil, false
	}

	return entry.results, true
}

// setInCache stores search results with TTL of 1 minute.
func (sc *searchCache) setInCache(key string, results []searchResult) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.entries[key] = cacheEntry{
		results:   results,
		expiresAt: time.Now().Add(searchCacheTTL),
	}
}

// renewCacheTTL extends the TTL of an existing cache entry by 1 more minute.
// Called when navigating pages (different --from value) to keep cache alive during pagination.
func (sc *searchCache) renewCacheTTL(key string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	entry, exists := sc.entries[key]
	if !exists {
		return
	}

	entry.expiresAt = time.Now().Add(searchCacheTTL)
	sc.entries[key] = entry
}

// GetCacheEntries returns all cache entries for testing purposes.
// This is exported for test introspection only.
func (sc *searchCache) GetCacheEntries() map[string]cacheEntry {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Return a copy to avoid exposing internal pointers.
	entries := make(map[string]cacheEntry, len(sc.entries))
	for k, v := range sc.entries {
		entries[k] = v
	}
	return entries
}
