package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"idx/internal/shared/config"
)

func TestDefaultIdxConfig_Search_ReturnsExpectedDefaults(t *testing.T) {
	t.Parallel()

	// Act
	cfg := config.DefaultIdxConfig()

	// Assert
	assert.Equal(t, "text", cfg.Search.Format)
	assert.Equal(t, 0, cfg.Search.Limit)
	assert.Equal(t, "AND", cfg.Search.Operator)
	assert.Equal(t, 0, cfg.Search.Context)
	assert.Equal(t, "", cfg.Search.Relaxation)
	assert.Equal(t, time.Minute, cfg.Search.CacheTTL)
	assert.Equal(t, 4, cfg.Search.MaxWorkers)
}

func TestDefaultIdxConfig_Watch_ReturnsExpectedDefaults(t *testing.T) {
	t.Parallel()

	// Act
	cfg := config.DefaultIdxConfig()

	// Assert
	assert.Equal(t, 750*time.Millisecond, cfg.Watch.Debounce)
}

func TestDefaultIdxConfig_BM25_ReturnsExpectedDefaults(t *testing.T) {
	t.Parallel()

	// Act
	cfg := config.DefaultIdxConfig()

	// Assert
	assert.Equal(t, 1.5, cfg.BM25.K1)
	assert.Equal(t, 0.75, cfg.BM25.B)
	assert.Equal(t, 3.0, cfg.BM25.ProximityWeight)
}

func TestDefaultIdxConfig_Log_ReturnsExpectedDefaults(t *testing.T) {
	t.Parallel()

	// Act
	cfg := config.DefaultIdxConfig()

	// Assert
	assert.Equal(t, "error", cfg.Log.Level)
}

func TestDefaultIdxConfig_Index_ReturnsEmptyNonNilIgnoreSlice(t *testing.T) {
	t.Parallel()

	// Act
	cfg := config.DefaultIdxConfig()

	// Assert
	assert.NotNil(t, cfg.Index.Ignore)
	assert.Empty(t, cfg.Index.Ignore)
}

func TestDefaultIdxConfig_ReturnsIndependentInstances(t *testing.T) {
	t.Parallel()

	// Arrange
	a := config.DefaultIdxConfig()
	b := config.DefaultIdxConfig()

	// Act
	a.Search.Format = "json"

	// Assert
	assert.NotEqual(t, "json", b.Search.Format, "mutation of one instance must not affect another")
}
