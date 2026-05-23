package domain

import "time"

// IdxConfig holds all project-level configuration for the idx tool.
// Populated from .idx.yml at the git root; missing keys fall back to defaults.
type IdxConfig struct {
	Search SearchConfig `yaml:"search"`
	Watch  WatchConfig  `yaml:"watch"`
	Index  IndexConfig  `yaml:"index"`
	BM25   BM25Config   `yaml:"bm25"`
	Log    LogConfig    `yaml:"log"`
}

// SearchConfig controls default search behaviour.
type SearchConfig struct {
	Format     string        `yaml:"format"`
	Size       int           `yaml:"size"`
	Operator   string        `yaml:"operator"`
	Context    int           `yaml:"context"`
	Relaxation string        `yaml:"relaxation"`
	CacheTTL   time.Duration `yaml:"cache_ttl"`
	MaxWorkers int           `yaml:"max_workers"`
}

// WatchConfig controls the file-watch loop.
type WatchConfig struct {
	Debounce time.Duration `yaml:"debounce"`
}

// IndexConfig controls what gets indexed.
type IndexConfig struct {
	Ignore []string `yaml:"ignore"`
}

// BM25Config holds BM25 relevance-tuning parameters.
type BM25Config struct {
	K1               float64 `yaml:"k1"`
	B                float64 `yaml:"b"`
	ProximityWeight  float64 `yaml:"proximity_weight"`
	PopularityWeight float64 `yaml:"popularity_weight"`
}

// LogConfig controls logging verbosity.
// Use the IDX_LOG_LEVEL environment variable (or set log.level in .idx.yml);
// env var takes precedence over the config file.
type LogConfig struct {
	Level string `yaml:"level"`
}

// DefaultIdxConfig returns the built-in defaults, identical to the constants
// previously hardcoded across the service layer.
func DefaultIdxConfig() IdxConfig {
	return IdxConfig{
		Search: SearchConfig{
			Format:     "text",
			Size:       0,
			Operator:   "AND",
			Context:    0,
			Relaxation: "",
			CacheTTL:   time.Minute,
			MaxWorkers: 4,
		},
		Watch: WatchConfig{
			Debounce: 750 * time.Millisecond,
		},
		Index: IndexConfig{
			Ignore: []string{},
		},
		BM25: BM25Config{
			K1:               1.5,
			B:                0.75,
			ProximityWeight:  3.0,
			PopularityWeight: 0.3,
		},
		Log: LogConfig{
			Level: "error",
		},
	}
}
