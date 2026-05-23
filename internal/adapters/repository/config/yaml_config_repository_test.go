package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"idx/internal/adapters/repository/config"
	"idx/internal/core/domain"
)

func TestYAMLConfigRepositoryFilePathReturnEmptyWhenNoFile(t *testing.T) {
	repo := config.NewYAMLConfigRepository()
	dir := t.TempDir()

	got := repo.FilePath(dir)
	if got != "" {
		t.Fatalf("expected empty path when .idx.yml absent, got %q", got)
	}
}

func TestYAMLConfigRepositoryFilePathReturnsAbsolutePathWhenFileExists(t *testing.T) {
	repo := config.NewYAMLConfigRepository()
	dir := t.TempDir()
	writeConfig(t, dir, "")

	got := repo.FilePath(dir)
	expected := filepath.Join(dir, ".idx.yml")
	if got != expected {
		t.Fatalf("expected path %q, got %q", expected, got)
	}
}

func TestYAMLConfigRepositoryLoadReturnsDefaultsWhenFileAbsent(t *testing.T) {
	repo := config.NewYAMLConfigRepository()
	dir := t.TempDir()

	cfg, overrides, err := repo.Load(dir)
	if err != nil {
		t.Fatalf("expected no error when file absent, got %v", err)
	}
	if len(overrides) != 0 {
		t.Fatalf("expected no overrides when file absent, got %v", overrides)
	}

	def := domain.DefaultIdxConfig()
	if cfg.Search.Format != def.Search.Format {
		t.Fatalf("expected default format %q, got %q", def.Search.Format, cfg.Search.Format)
	}
	if cfg.BM25.K1 != def.BM25.K1 {
		t.Fatalf("expected default bm25.k1 %v, got %v", def.BM25.K1, cfg.BM25.K1)
	}
}

func TestYAMLConfigRepositoryLoadParsesExplicitValues(t *testing.T) {
	repo := config.NewYAMLConfigRepository()
	dir := t.TempDir()
	writeConfig(t, dir, `
search:
  format: json
  size: 20
  operator: OR
bm25:
  k1: 1.2
`)

	cfg, _, err := repo.Load(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Search.Format != "json" {
		t.Fatalf("expected search.format json, got %q", cfg.Search.Format)
	}
	if cfg.Search.Size != 20 {
		t.Fatalf("expected search.size 20, got %d", cfg.Search.Size)
	}
	if cfg.Search.Operator != "OR" {
		t.Fatalf("expected search.operator OR, got %q", cfg.Search.Operator)
	}
	if cfg.BM25.K1 != 1.2 {
		t.Fatalf("expected bm25.k1 1.2, got %v", cfg.BM25.K1)
	}
}

func TestYAMLConfigRepositoryLoadTracksExactOverrides(t *testing.T) {
	repo := config.NewYAMLConfigRepository()
	dir := t.TempDir()
	writeConfig(t, dir, `
search:
  format: json
  size: 5
bm25:
  k1: 1.2
`)

	_, overrides, err := repo.Load(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	overrideSet := make(map[string]bool)
	for _, k := range overrides {
		overrideSet[k] = true
	}

	for _, expected := range []string{"search.format", "search.size", "bm25.k1"} {
		if !overrideSet[expected] {
			t.Errorf("expected override %q to be tracked, got overrides: %v", expected, overrides)
		}
	}
	if overrideSet["search.operator"] {
		t.Errorf("expected search.operator not tracked (not in file), but it was")
	}
}

func TestYAMLConfigRepositoryLoadKeepsDefaultsForUnsetKeys(t *testing.T) {
	repo := config.NewYAMLConfigRepository()
	dir := t.TempDir()
	writeConfig(t, dir, `
search:
  format: json
`)

	cfg, _, err := repo.Load(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	def := domain.DefaultIdxConfig()
	if cfg.Search.Operator != def.Search.Operator {
		t.Fatalf("expected default operator %q for unset key, got %q", def.Search.Operator, cfg.Search.Operator)
	}
	if cfg.BM25.B != def.BM25.B {
		t.Fatalf("expected default bm25.b %v for unset key, got %v", def.BM25.B, cfg.BM25.B)
	}
}

func TestYAMLConfigRepositoryLoadParsesDurationStrings(t *testing.T) {
	repo := config.NewYAMLConfigRepository()
	dir := t.TempDir()
	writeConfig(t, dir, `
search:
  cache_ttl: 2m
watch:
  debounce: 250ms
`)

	cfg, overrides, err := repo.Load(dir)
	if err != nil {
		t.Fatalf("expected no error parsing durations, got %v", err)
	}

	if cfg.Search.CacheTTL != 2*time.Minute {
		t.Fatalf("expected search.cache_ttl 2m, got %v", cfg.Search.CacheTTL)
	}
	if cfg.Watch.Debounce != 250*time.Millisecond {
		t.Fatalf("expected watch.debounce 250ms, got %v", cfg.Watch.Debounce)
	}

	overrideSet := make(map[string]bool)
	for _, k := range overrides {
		overrideSet[k] = true
	}
	if !overrideSet["search.cache_ttl"] {
		t.Error("expected search.cache_ttl to be tracked as override")
	}
	if !overrideSet["watch.debounce"] {
		t.Error("expected watch.debounce to be tracked as override")
	}
}

func TestYAMLConfigRepositoryLoadReturnsErrorForInvalidDuration(t *testing.T) {
	repo := config.NewYAMLConfigRepository()
	dir := t.TempDir()
	writeConfig(t, dir, `
watch:
  debounce: "not-a-duration"
`)

	_, _, err := repo.Load(dir)
	if err == nil {
		t.Fatal("expected error for invalid duration string, got nil")
	}
}

func TestYAMLConfigRepositoryLoadReturnsErrorForInvalidYAML(t *testing.T) {
	repo := config.NewYAMLConfigRepository()
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, ".idx.yml"), []byte(":\tinvalid: yaml::\n"), 0o600); err != nil {
		t.Fatalf("failed to write bad config: %v", err)
	}

	_, _, err := repo.Load(dir)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestYAMLConfigRepositoryLoadParsesIgnorePatterns(t *testing.T) {
	repo := config.NewYAMLConfigRepository()
	dir := t.TempDir()
	writeConfig(t, dir, `
index:
  ignore:
    - vendor/
    - "*.pb.go"
`)

	cfg, overrides, err := repo.Load(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(cfg.Index.Ignore) != 2 {
		t.Fatalf("expected 2 ignore patterns, got %d: %v", len(cfg.Index.Ignore), cfg.Index.Ignore)
	}
	if cfg.Index.Ignore[0] != "vendor/" {
		t.Fatalf("expected first pattern vendor/, got %q", cfg.Index.Ignore[0])
	}

	overrideSet := make(map[string]bool)
	for _, k := range overrides {
		overrideSet[k] = true
	}
	if !overrideSet["index.ignore"] {
		t.Error("expected index.ignore to be tracked as override")
	}
}

func TestYAMLConfigRepositoryLoadParsesLogLevel(t *testing.T) {
	repo := config.NewYAMLConfigRepository()
	dir := t.TempDir()
	writeConfig(t, dir, `
log:
  level: debug
`)

	cfg, overrides, err := repo.Load(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cfg.Log.Level != "debug" {
		t.Fatalf("expected log.level debug, got %q", cfg.Log.Level)
	}

	overrideSet := make(map[string]bool)
	for _, k := range overrides {
		overrideSet[k] = true
	}
	if !overrideSet["log.level"] {
		t.Error("expected log.level to be tracked as override")
	}
}

func TestYAMLConfigRepositoryLoadEmptyFileReturnsDefaults(t *testing.T) {
	repo := config.NewYAMLConfigRepository()
	dir := t.TempDir()
	writeConfig(t, dir, "")

	cfg, overrides, err := repo.Load(dir)
	if err != nil {
		t.Fatalf("expected no error for empty file, got %v", err)
	}
	if len(overrides) != 0 {
		t.Fatalf("expected no overrides for empty file, got %v", overrides)
	}

	def := domain.DefaultIdxConfig()
	if cfg.Search.Format != def.Search.Format {
		t.Fatalf("expected default format for empty file, got %q", cfg.Search.Format)
	}
}

// writeConfig writes content to .idx.yml inside dir.
func writeConfig(t *testing.T, dir, content string) {
	t.Helper()
	path := filepath.Join(dir, ".idx.yml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}
}
