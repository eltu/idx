//go:build race

package search_test

import "time"

func defaultConcurrencyTestSettings() concurrencyTestSettings {
	// Race detector instrumentation adds significant overhead; keep enough load
	// to exercise concurrent sync/search without hitting CI wall-clock limits.
	return concurrencyTestSettings{
		files:            60,
		subdirs:          4,
		filesPerDir:      16,
		syncIterations:   70,
		searchWorkers:    4,
		searchIterations: 110,
		timeout:          45 * time.Second,
	}
}
