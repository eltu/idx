package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"idx/internal/shared/config"
)

func TestYAMLRepository_FilePath_ReturnsEmptyWhenFileAbsent(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := config.NewYAMLRepository()
	dir := t.TempDir()

	// Act
	got := repo.FilePath(dir)

	// Assert
	assert.Empty(t, got)
}

func TestYAMLRepository_FilePath_ReturnsAbsolutePathWhenFileExists(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := config.NewYAMLRepository()
	dir := t.TempDir()
	writeYAMLConfig(t, dir, "")

	// Act
	got := repo.FilePath(dir)

	// Assert
	assert.Equal(t, filepath.Join(dir, ".idx.yml"), got)
}

func TestYAMLRepository_Load_ReturnsDefaultsWhenFileAbsent(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := config.NewYAMLRepository()
	dir := t.TempDir()
	def := config.DefaultIdxConfig()

	// Act
	cfg, overrides, err := repo.Load(dir)

	// Assert
	require.NoError(t, err)
	assert.Empty(t, overrides)
	assert.Equal(t, def.Search.Format, cfg.Search.Format)
	assert.Equal(t, def.BM25.K1, cfg.BM25.K1)
}

func TestYAMLRepository_Load_ParsesExplicitValues(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := config.NewYAMLRepository()
	dir := t.TempDir()
	writeYAMLConfig(t, dir, `
search:
  format: json
  size: 20
  operator: OR
bm25:
  k1: 1.2
`)

	// Act
	cfg, _, err := repo.Load(dir)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "json", cfg.Search.Format)
	assert.Equal(t, 20, cfg.Search.Limit)
	assert.Equal(t, "OR", cfg.Search.Operator)
	assert.Equal(t, 1.2, cfg.BM25.K1)
}

func TestYAMLRepository_Load_TracksExplicitOverrides(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := config.NewYAMLRepository()
	dir := t.TempDir()
	writeYAMLConfig(t, dir, `
search:
  format: json
  size: 5
bm25:
  k1: 1.2
`)

	// Act
	_, overrides, err := repo.Load(dir)

	// Assert
	require.NoError(t, err)
	overrideSet := overrideMap(overrides)
	for _, expected := range []string{"search.format", "search.limit", "bm25.k1"} {
		assert.True(t, overrideSet[expected], "expected override %q to be tracked", expected)
	}
	assert.False(t, overrideSet["search.operator"], "search.operator should not be tracked when not in file")
}

func TestYAMLRepository_Load_SearchLimitKey_ParsesCorrectly(t *testing.T) {
	t.Parallel()

	// Arrange — new canonical key name
	repo := config.NewYAMLRepository()
	dir := t.TempDir()
	writeYAMLConfig(t, dir, `
search:
  limit: 15
`)

	// Act
	cfg, overrides, err := repo.Load(dir)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 15, cfg.Search.Limit)
	overrideSet := overrideMap(overrides)
	assert.True(t, overrideSet["search.limit"], "expected search.limit to be tracked as override")
}

func TestYAMLRepository_Load_SearchSizeKey_BackwardCompat(t *testing.T) {
	t.Parallel()

	// Arrange — old YAML key; must still be read after the rename to 'limit'
	repo := config.NewYAMLRepository()
	dir := t.TempDir()
	writeYAMLConfig(t, dir, `
search:
  size: 7
`)

	// Act
	cfg, overrides, err := repo.Load(dir)

	// Assert — value read from old key, reported under new key name
	require.NoError(t, err)
	assert.Equal(t, 7, cfg.Search.Limit)
	overrideSet := overrideMap(overrides)
	assert.True(t, overrideSet["search.limit"], "deprecated size: key should be reported as search.limit override")
}

func TestYAMLRepository_Load_KeepsDefaultsForUnsetKeys(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := config.NewYAMLRepository()
	dir := t.TempDir()
	writeYAMLConfig(t, dir, `
search:
  format: json
`)
	def := config.DefaultIdxConfig()

	// Act
	cfg, _, err := repo.Load(dir)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, def.Search.Operator, cfg.Search.Operator)
	assert.Equal(t, def.BM25.B, cfg.BM25.B)
}

func TestYAMLRepository_Load_ParsesDurationStrings(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := config.NewYAMLRepository()
	dir := t.TempDir()
	writeYAMLConfig(t, dir, `
search:
  cache_ttl: 2m
watch:
  debounce: 250ms
`)

	// Act
	cfg, overrides, err := repo.Load(dir)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 2*time.Minute, cfg.Search.CacheTTL)
	assert.Equal(t, 250*time.Millisecond, cfg.Watch.Debounce)
	overrideSet := overrideMap(overrides)
	assert.True(t, overrideSet["search.cache_ttl"])
	assert.True(t, overrideSet["watch.debounce"])
}

func TestYAMLRepository_Load_ReturnsErrorForInvalidDuration(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := config.NewYAMLRepository()
	dir := t.TempDir()
	writeYAMLConfig(t, dir, `
watch:
  debounce: "not-a-duration"
`)

	// Act
	_, _, err := repo.Load(dir)

	// Assert
	require.Error(t, err)
}

func TestYAMLRepository_Load_ReturnsErrorForInvalidYAML(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := config.NewYAMLRepository()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".idx.yml"), []byte(":\tinvalid: yaml::\n"), 0o600))

	// Act
	_, _, err := repo.Load(dir)

	// Assert
	require.Error(t, err)
}

func TestYAMLRepository_Load_ParsesIgnorePatterns(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := config.NewYAMLRepository()
	dir := t.TempDir()
	writeYAMLConfig(t, dir, `
index:
  ignore:
    - vendor/
    - "*.pb.go"
`)

	// Act
	cfg, overrides, err := repo.Load(dir)

	// Assert
	require.NoError(t, err)
	require.Len(t, cfg.Index.Ignore, 2)
	assert.Equal(t, "vendor/", cfg.Index.Ignore[0])
	overrideSet := overrideMap(overrides)
	assert.True(t, overrideSet["index.ignore"])
}

func TestYAMLRepository_Load_ParsesLogLevel(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := config.NewYAMLRepository()
	dir := t.TempDir()
	writeYAMLConfig(t, dir, `
log:
  level: debug
`)

	// Act
	cfg, overrides, err := repo.Load(dir)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "debug", cfg.Log.Level)
	overrideSet := overrideMap(overrides)
	assert.True(t, overrideSet["log.level"])
}

func TestYAMLRepository_Load_EmptyFileReturnsDefaults(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := config.NewYAMLRepository()
	dir := t.TempDir()
	writeYAMLConfig(t, dir, "")
	def := config.DefaultIdxConfig()

	// Act
	cfg, overrides, err := repo.Load(dir)

	// Assert
	require.NoError(t, err)
	assert.Empty(t, overrides)
	assert.Equal(t, def.Search.Format, cfg.Search.Format)
}

func writeYAMLConfig(t *testing.T, dir, content string) {
	t.Helper()
	path := filepath.Join(dir, ".idx.yml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}
