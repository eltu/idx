//go:build !race

package search_test

import "time"

func defaultConcurrencyTestSettings() concurrencyTestSettings {
	return concurrencyTestSettings{
		files:            120,
		subdirs:          6,
		filesPerDir:      30,
		syncIterations:   140,
		searchWorkers:    6,
		searchIterations: 260,
		timeout:          20 * time.Second,
	}
}
