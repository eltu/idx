package config_test

import (
	"testing"

	"idx/internal/shared/config"
)

func TestYAMLRepositoryLoadParsesSearchContextAndRelaxation(t *testing.T) {
	repo := config.NewYAMLRepository()
	dir := t.TempDir()
	writeYAMLConfig(t, dir, `
search:
  context: 4
  relaxation: "5"
  max_workers: 8
`)

	cfg, overrides, err := repo.Load(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Search.Context != 4 {
		t.Errorf("expected search.context 4, got %d", cfg.Search.Context)
	}
	if cfg.Search.Relaxation != "5" {
		t.Errorf("expected search.relaxation \"5\", got %q", cfg.Search.Relaxation)
	}
	if cfg.Search.MaxWorkers != 8 {
		t.Errorf("expected search.max_workers 8, got %d", cfg.Search.MaxWorkers)
	}

	overrideSet := overrideMap(overrides)
	for _, key := range []string{"search.context", "search.relaxation", "search.max_workers"} {
		if !overrideSet[key] {
			t.Errorf("expected %q in overrides, got: %v", key, overrides)
		}
	}
}

func TestYAMLRepositoryLoadParsesBM25BAndProximityWeight(t *testing.T) {
	repo := config.NewYAMLRepository()
	dir := t.TempDir()
	writeYAMLConfig(t, dir, `
bm25:
  b: 0.85
  proximity_weight: 0.3
`)

	cfg, overrides, err := repo.Load(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.BM25.B != 0.85 {
		t.Errorf("expected bm25.b 0.85, got %v", cfg.BM25.B)
	}
	if cfg.BM25.ProximityWeight != 0.3 {
		t.Errorf("expected bm25.proximity_weight 0.3, got %v", cfg.BM25.ProximityWeight)
	}

	overrideSet := overrideMap(overrides)
	for _, key := range []string{"bm25.b", "bm25.proximity_weight"} {
		if !overrideSet[key] {
			t.Errorf("expected %q in overrides, got: %v", key, overrides)
		}
	}
}

func TestYAMLRepositoryLoadNilBM25SectionSkipped(t *testing.T) {
	repo := config.NewYAMLRepository()
	dir := t.TempDir()
	writeYAMLConfig(t, dir, `
search:
  format: json
`)

	_, overrides, err := repo.Load(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	overrideSet := overrideMap(overrides)
	if overrideSet["bm25.k1"] || overrideSet["bm25.b"] {
		t.Errorf("expected no bm25 overrides when section absent, got: %v", overrides)
	}
}

func TestYAMLRepositoryLoadNilLogSectionSkipped(t *testing.T) {
	repo := config.NewYAMLRepository()
	dir := t.TempDir()
	writeYAMLConfig(t, dir, `
search:
  format: json
`)

	_, overrides, err := repo.Load(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	overrideSet := overrideMap(overrides)
	if overrideSet["log.level"] {
		t.Errorf("expected no log.level override when section absent, got: %v", overrides)
	}
}

func TestYAMLRepositoryLoadNilWatchSectionReturnsNil(t *testing.T) {
	repo := config.NewYAMLRepository()
	dir := t.TempDir()
	writeYAMLConfig(t, dir, `
bm25:
  k1: 1.5
`)

	_, overrides, err := repo.Load(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	overrideSet := overrideMap(overrides)
	if overrideSet["watch.debounce"] {
		t.Errorf("expected no watch.debounce override when section absent, got: %v", overrides)
	}
}

func TestYAMLRepositoryLoadNilIndexSectionReturnsNil(t *testing.T) {
	repo := config.NewYAMLRepository()
	dir := t.TempDir()
	writeYAMLConfig(t, dir, `
bm25:
  k1: 1.5
`)

	_, overrides, err := repo.Load(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	overrideSet := overrideMap(overrides)
	if overrideSet["index.ignore"] {
		t.Errorf("expected no index.ignore override when section absent, got: %v", overrides)
	}
}

func overrideMap(overrides []string) map[string]bool {
	m := make(map[string]bool, len(overrides))
	for _, k := range overrides {
		m[k] = true
	}
	return m
}
