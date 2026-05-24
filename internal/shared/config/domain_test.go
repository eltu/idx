package config_test

import (
	"testing"
	"time"

	"idx/internal/shared/config"
)

func TestDefaultIdxConfigSearchDefaults(t *testing.T) {
	cfg := config.DefaultIdxConfig()

	if cfg.Search.Format != "text" {
		t.Fatalf("expected search.format text, got %q", cfg.Search.Format)
	}
	if cfg.Search.Size != 0 {
		t.Fatalf("expected search.size 0 (unlimited), got %d", cfg.Search.Size)
	}
	if cfg.Search.Operator != "AND" {
		t.Fatalf("expected search.operator AND, got %q", cfg.Search.Operator)
	}
	if cfg.Search.Context != 0 {
		t.Fatalf("expected search.context 0, got %d", cfg.Search.Context)
	}
	if cfg.Search.Relaxation != "" {
		t.Fatalf("expected search.relaxation empty, got %q", cfg.Search.Relaxation)
	}
	if cfg.Search.CacheTTL != time.Minute {
		t.Fatalf("expected search.cache_ttl 1m, got %v", cfg.Search.CacheTTL)
	}
	if cfg.Search.MaxWorkers != 4 {
		t.Fatalf("expected search.max_workers 4, got %d", cfg.Search.MaxWorkers)
	}
}

func TestDefaultIdxConfigWatchDefaults(t *testing.T) {
	cfg := config.DefaultIdxConfig()

	if cfg.Watch.Debounce != 750*time.Millisecond {
		t.Fatalf("expected watch.debounce 750ms, got %v", cfg.Watch.Debounce)
	}
}

func TestDefaultIdxConfigBM25Defaults(t *testing.T) {
	cfg := config.DefaultIdxConfig()

	if cfg.BM25.K1 != 1.5 {
		t.Fatalf("expected bm25.k1 1.5, got %v", cfg.BM25.K1)
	}
	if cfg.BM25.B != 0.75 {
		t.Fatalf("expected bm25.b 0.75, got %v", cfg.BM25.B)
	}
	if cfg.BM25.ProximityWeight != 3.0 {
		t.Fatalf("expected bm25.proximity_weight 3.0, got %v", cfg.BM25.ProximityWeight)
	}
}

func TestDefaultIdxConfigLogDefaults(t *testing.T) {
	cfg := config.DefaultIdxConfig()

	if cfg.Log.Level != "error" {
		t.Fatalf("expected log.level error, got %q", cfg.Log.Level)
	}
}

func TestDefaultIdxConfigIndexDefaults(t *testing.T) {
	cfg := config.DefaultIdxConfig()

	if cfg.Index.Ignore == nil {
		t.Fatal("expected index.ignore to be non-nil slice, got nil")
	}
	if len(cfg.Index.Ignore) != 0 {
		t.Fatalf("expected empty index.ignore, got %v", cfg.Index.Ignore)
	}
}

func TestDefaultIdxConfigReturnsIndependentInstances(t *testing.T) {
	a := config.DefaultIdxConfig()
	b := config.DefaultIdxConfig()

	a.Search.Format = "json"
	if b.Search.Format == "json" {
		t.Fatal("expected DefaultIdxConfig to return independent instances, but mutation affected other copy")
	}
}
