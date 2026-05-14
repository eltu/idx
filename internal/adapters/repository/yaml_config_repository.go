package repository

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
	Exclude []string `yaml:"exclude"`
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

	if s := raw.Search; s != nil {
		if s.Format != nil {
			cfg.Search.Format = *s.Format
			overrides = append(overrides, "search.format")
		}
		if s.Size != nil {
			cfg.Search.Size = *s.Size
			overrides = append(overrides, "search.size")
		}
		if s.Operator != nil {
			cfg.Search.Operator = *s.Operator
			overrides = append(overrides, "search.operator")
		}
		if s.Context != nil {
			cfg.Search.Context = *s.Context
			overrides = append(overrides, "search.context")
		}
		if s.Relaxation != nil {
			cfg.Search.Relaxation = *s.Relaxation
			overrides = append(overrides, "search.relaxation")
		}
		if s.CacheTTL != nil {
			d, err := time.ParseDuration(*s.CacheTTL)
			if err != nil {
				return nil, fmt.Errorf("invalid search.cache_ttl %q: expected a Go duration like 60s, 2m, 500ms", *s.CacheTTL)
			}
			cfg.Search.CacheTTL = d
			overrides = append(overrides, "search.cache_ttl")
		}
		if s.MaxWorkers != nil {
			cfg.Search.MaxWorkers = *s.MaxWorkers
			overrides = append(overrides, "search.max_workers")
		}
	}

	if w := raw.Watch; w != nil {
		if w.Debounce != nil {
			d, err := time.ParseDuration(*w.Debounce)
			if err != nil {
				return nil, fmt.Errorf("invalid watch.debounce %q: expected a Go duration like 750ms, 1s, 2s", *w.Debounce)
			}
			cfg.Watch.Debounce = d
			overrides = append(overrides, "watch.debounce")
		}
	}

	if i := raw.Index; i != nil {
		if i.Exclude != nil {
			cfg.Index.Exclude = i.Exclude
			overrides = append(overrides, "index.exclude")
		}
	}

	if b := raw.BM25; b != nil {
		if b.K1 != nil {
			cfg.BM25.K1 = *b.K1
			overrides = append(overrides, "bm25.k1")
		}
		if b.B != nil {
			cfg.BM25.B = *b.B
			overrides = append(overrides, "bm25.b")
		}
		if b.ProximityWeight != nil {
			cfg.BM25.ProximityWeight = *b.ProximityWeight
			overrides = append(overrides, "bm25.proximity_weight")
		}
	}

	if l := raw.Log; l != nil {
		if l.Level != nil {
			cfg.Log.Level = *l.Level
			overrides = append(overrides, "log.level")
		}
	}

	return overrides, nil
}
