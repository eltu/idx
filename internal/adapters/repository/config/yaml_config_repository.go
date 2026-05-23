package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"idx/internal/core/domain"
)

const idxConfigFileName = ".idx.yml"

// YAMLConfigRepository reads .idx.yml from the project root.
// Example: cfg, overrides, err := NewYAMLConfigRepository().Load(projectRoot).
type YAMLConfigRepository struct{}

func NewYAMLConfigRepository() *YAMLConfigRepository {
	return &YAMLConfigRepository{}
}

func (r *YAMLConfigRepository) FilePath(projectRoot string) string {
	path := filepath.Join(projectRoot, idxConfigFileName)
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return path
}

// Load reads .idx.yml, merges explicit values over the defaults, and returns
// a flat list of keys that were explicitly set (e.g. "search.format").
func (r *YAMLConfigRepository) Load(projectRoot string) (domain.IdxConfig, []string, error) {
	cfg := domain.DefaultIdxConfig()

	path := filepath.Join(projectRoot, idxConfigFileName)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil, nil
	}
	if err != nil {
		return cfg, nil, fmt.Errorf("failed to read config file %q: %w", path, err)
	}

	var raw yamlIdxConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return cfg, nil, fmt.Errorf("invalid %s at %q: %w", idxConfigFileName, path, err)
	}

	overrides, err := applyYAMLOverrides(&cfg, raw)
	if err != nil {
		return domain.DefaultIdxConfig(), nil, fmt.Errorf("invalid %s at %q: %w", idxConfigFileName, path, err)
	}

	return cfg, overrides, nil
}

// yamlIdxConfig uses pointer fields so absent keys remain nil, enabling
// precise detection of which values were explicitly set in the file.
type yamlIdxConfig struct {
	Search *yamlSearchConfig `yaml:"search"`
	Watch  *yamlWatchConfig  `yaml:"watch"`
	Index  *yamlIndexConfig  `yaml:"index"`
	BM25   *yamlBM25Config   `yaml:"bm25"`
	Log    *yamlLogConfig    `yaml:"log"`
}

type yamlSearchConfig struct {
	Format     *string `yaml:"format"`
	Size       *int    `yaml:"size"`
	Operator   *string `yaml:"operator"`
	Context    *int    `yaml:"context"`
	Relaxation *string `yaml:"relaxation"`
	CacheTTL   *string `yaml:"cache_ttl"`
	MaxWorkers *int    `yaml:"max_workers"`
}

type yamlWatchConfig struct {
	Debounce *string `yaml:"debounce"`
}

type yamlIndexConfig struct {
	Ignore []string `yaml:"ignore"`
}

type yamlBM25Config struct {
	K1              *float64 `yaml:"k1"`
	B               *float64 `yaml:"b"`
	ProximityWeight *float64 `yaml:"proximity_weight"`
}

type yamlLogConfig struct {
	Level *string `yaml:"level"`
}

func applyYAMLOverrides(cfg *domain.IdxConfig, raw yamlIdxConfig) ([]string, error) {
	var overrides []string

	if raw.Search != nil {
		keys, err := applySearchOverrides(&cfg.Search, raw.Search)
		if err != nil {
			return nil, err
		}
		overrides = append(overrides, keys...)
	}

	if raw.Watch != nil {
		keys, err := applyWatchOverrides(&cfg.Watch, raw.Watch)
		if err != nil {
			return nil, err
		}
		overrides = append(overrides, keys...)
	}

	if raw.Index != nil {
		overrides = append(overrides, applyIndexOverrides(&cfg.Index, raw.Index)...)
	}

	if raw.BM25 != nil {
		overrides = append(overrides, applyBM25Overrides(&cfg.BM25, raw.BM25)...)
	}

	if raw.Log != nil {
		overrides = append(overrides, applyLogOverrides(&cfg.Log, raw.Log)...)
	}

	return overrides, nil
}

func applySearchOverrides(cfg *domain.SearchConfig, s *yamlSearchConfig) ([]string, error) {
	var keys []string

	if s.Format != nil {
		cfg.Format = *s.Format
		keys = append(keys, "search.format")
	}
	if s.Size != nil {
		cfg.Size = *s.Size
		keys = append(keys, "search.size")
	}
	if s.Operator != nil {
		cfg.Operator = *s.Operator
		keys = append(keys, "search.operator")
	}
	if s.Context != nil {
		cfg.Context = *s.Context
		keys = append(keys, "search.context")
	}
	if s.Relaxation != nil {
		cfg.Relaxation = *s.Relaxation
		keys = append(keys, "search.relaxation")
	}
	if s.CacheTTL != nil {
		d, err := time.ParseDuration(*s.CacheTTL)
		if err != nil {
			return nil, fmt.Errorf("invalid search.cache_ttl %q: expected a Go duration like 60s, 2m, 500ms", *s.CacheTTL)
		}
		cfg.CacheTTL = d
		keys = append(keys, "search.cache_ttl")
	}
	if s.MaxWorkers != nil {
		cfg.MaxWorkers = *s.MaxWorkers
		keys = append(keys, "search.max_workers")
	}

	return keys, nil
}

func applyWatchOverrides(cfg *domain.WatchConfig, w *yamlWatchConfig) ([]string, error) {
	if w.Debounce == nil {
		return nil, nil
	}

	d, err := time.ParseDuration(*w.Debounce)
	if err != nil {
		return nil, fmt.Errorf("invalid watch.debounce %q: expected a Go duration like 750ms, 1s, 2s", *w.Debounce)
	}

	cfg.Debounce = d
	return []string{"watch.debounce"}, nil
}

func applyIndexOverrides(cfg *domain.IndexConfig, i *yamlIndexConfig) []string {
	if i.Ignore == nil {
		return nil
	}
	cfg.Ignore = i.Ignore
	return []string{"index.ignore"}
}

func applyBM25Overrides(cfg *domain.BM25Config, b *yamlBM25Config) []string {
	var keys []string

	if b.K1 != nil {
		cfg.K1 = *b.K1
		keys = append(keys, "bm25.k1")
	}
	if b.B != nil {
		cfg.B = *b.B
		keys = append(keys, "bm25.b")
	}
	if b.ProximityWeight != nil {
		cfg.ProximityWeight = *b.ProximityWeight
		keys = append(keys, "bm25.proximity_weight")
	}

	return keys
}

func applyLogOverrides(cfg *domain.LogConfig, l *yamlLogConfig) []string {
	if l.Level == nil {
		return nil
	}
	cfg.Level = *l.Level
	return []string{"log.level"}
}
