package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"idx/internal/shared/config"
)

func TestYAMLRepository_Load_ParsesSearchContextAndRelaxation(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := config.NewYAMLRepository()
	dir := t.TempDir()
	writeYAMLConfig(t, dir, `
search:
  context: 4
  relaxation: 5
  max_workers: 8
`)

	// Act
	cfg, overrides, err := repo.Load(dir)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 4, cfg.Search.Context)
	assert.Equal(t, 5, cfg.Search.Relaxation)
	assert.Equal(t, 8, cfg.Search.MaxWorkers)
	overrideSet := overrideMap(overrides)
	for _, key := range []string{"search.context", "search.relaxation", "search.max_workers"} {
		assert.True(t, overrideSet[key], "expected %q in overrides, got: %v", key, overrides)
	}
}

func TestYAMLRepository_Load_ParsesBM25BAndProximityWeight(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := config.NewYAMLRepository()
	dir := t.TempDir()
	writeYAMLConfig(t, dir, `
bm25:
  b: 0.85
  proximity_weight: 0.3
`)

	// Act
	cfg, overrides, err := repo.Load(dir)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 0.85, cfg.BM25.B)
	assert.Equal(t, 0.3, cfg.BM25.ProximityWeight)
	overrideSet := overrideMap(overrides)
	for _, key := range []string{"bm25.b", "bm25.proximity_weight"} {
		assert.True(t, overrideSet[key], "expected %q in overrides, got: %v", key, overrides)
	}
}

func TestYAMLRepository_Load_SkipsBM25OverridesWhenSectionAbsent(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := config.NewYAMLRepository()
	dir := t.TempDir()
	writeYAMLConfig(t, dir, `
search:
  format: json
`)

	// Act
	_, overrides, err := repo.Load(dir)

	// Assert
	require.NoError(t, err)
	overrideSet := overrideMap(overrides)
	assert.False(t, overrideSet["bm25.k1"], "expected no bm25 overrides when section absent")
	assert.False(t, overrideSet["bm25.b"], "expected no bm25 overrides when section absent")
}

func TestYAMLRepository_Load_SkipsLogOverridesWhenSectionAbsent(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := config.NewYAMLRepository()
	dir := t.TempDir()
	writeYAMLConfig(t, dir, `
search:
  format: json
`)

	// Act
	_, overrides, err := repo.Load(dir)

	// Assert
	require.NoError(t, err)
	overrideSet := overrideMap(overrides)
	assert.False(t, overrideSet["log.level"], "expected no log.level override when section absent")
}

func TestYAMLRepository_Load_SkipsWatchOverridesWhenSectionAbsent(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := config.NewYAMLRepository()
	dir := t.TempDir()
	writeYAMLConfig(t, dir, `
bm25:
  k1: 1.5
`)

	// Act
	_, overrides, err := repo.Load(dir)

	// Assert
	require.NoError(t, err)
	overrideSet := overrideMap(overrides)
	assert.False(t, overrideSet["watch.debounce"], "expected no watch.debounce override when section absent")
}

func TestYAMLRepository_Load_SkipsIndexOverridesWhenSectionAbsent(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := config.NewYAMLRepository()
	dir := t.TempDir()
	writeYAMLConfig(t, dir, `
bm25:
  k1: 1.5
`)

	// Act
	_, overrides, err := repo.Load(dir)

	// Assert
	require.NoError(t, err)
	overrideSet := overrideMap(overrides)
	assert.False(t, overrideSet["index.ignore"], "expected no index.ignore override when section absent")
}

func TestYAMLRepository_Load_ParsesBM25PopularityWeight(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := config.NewYAMLRepository()
	dir := t.TempDir()
	writeYAMLConfig(t, dir, `
bm25:
  popularity_weight: 0.7
`)

	// Act
	cfg, overrides, err := repo.Load(dir)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 0.7, cfg.BM25.PopularityWeight)
	overrideSet := overrideMap(overrides)
	assert.True(t, overrideSet["bm25.popularity_weight"],
		"expected bm25.popularity_weight to be tracked as override, got: %v", overrides)
}

func TestYAMLRepository_Load_SkipsBM25PopularityWeightWhenAbsent(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := config.NewYAMLRepository()
	dir := t.TempDir()
	writeYAMLConfig(t, dir, `
bm25:
  k1: 1.2
`)

	// Act
	_, overrides, err := repo.Load(dir)

	// Assert
	require.NoError(t, err)
	overrideSet := overrideMap(overrides)
	assert.False(t, overrideSet["bm25.popularity_weight"],
		"expected bm25.popularity_weight NOT in overrides when absent from file")
}

func overrideMap(overrides []string) map[string]bool {
	m := make(map[string]bool, len(overrides))
	for _, k := range overrides {
		m[k] = true
	}
	return m
}
