package config_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"idx/internal/shared/config"
)

// --- AllKeys ---

func TestAllKeys_ReturnsFourteenKeys(t *testing.T) {
	t.Parallel()
	assert.Len(t, config.AllKeys(), 13)
}

func TestAllKeys_ContainsAllExpectedKeys(t *testing.T) {
	t.Parallel()
	keys := config.AllKeys()
	for _, required := range []string{"search.format", "search.size", "bm25.k1", "bm25.b", "log.level"} {
		assert.Contains(t, keys, required)
	}
}

// --- FieldValue ---

func TestFieldValue_SearchFormat_ReturnsValue(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultIdxConfig()
	cfg.Search.Format = "json"
	assert.Equal(t, "json", config.FieldValue(cfg, config.KeySearchFormat))
}

func TestFieldValue_BM25K1_FormatsFloat(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultIdxConfig()
	assert.Equal(t, config.FormatFloat(cfg.BM25.K1), config.FieldValue(cfg, config.KeyBM25K1))
}

func TestFieldValue_IndexIgnore_FormatsPatterns(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultIdxConfig()
	cfg.Index.Ignore = []string{"vendor/", "*.pb.go"}
	assert.Equal(t, "[vendor/, *.pb.go]", config.FieldValue(cfg, config.KeyIndexIgnore))
}

func TestFieldValue_UnknownKey_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	assert.Empty(t, config.FieldValue(config.DefaultIdxConfig(), "unknown.key"))
}

// --- DefaultFieldValue ---

func TestDefaultFieldValue_SearchFormat_ReturnsText(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "text", config.DefaultFieldValue(config.KeySearchFormat))
}

func TestDefaultFieldValue_AllKeys_HaveNonEmptyDefaults(t *testing.T) {
	t.Parallel()
	// relaxation default is intentionally empty — skip it
	for _, key := range config.AllKeys() {
		if key == config.KeySearchRelaxation {
			continue
		}
		assert.NotEmpty(t, config.DefaultFieldValue(key), "expected non-empty default for key %q", key)
	}
}

// --- FormatFloat ---

func TestFormatFloat_Decimal_RendersCorrectly(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "1.5", config.FormatFloat(1.5))
}

func TestFormatFloat_Zero_RendersZero(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "0", config.FormatFloat(0))
}

// --- FormatIgnore ---

func TestFormatIgnore_EmptySlice_ReturnsBrackets(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "[]", config.FormatIgnore(nil))
	assert.Equal(t, "[]", config.FormatIgnore([]string{}))
}

func TestFormatIgnore_OnePattern_ReturnsBracketedPattern(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "[vendor/]", config.FormatIgnore([]string{"vendor/"}))
}

func TestFormatIgnore_MultiplePatterns_JoinsWithComma(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "[vendor/, *.pb.go]", config.FormatIgnore([]string{"vendor/", "*.pb.go"}))
}

// --- PadRight ---

func TestPadRight_PadsShortString(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "abc   ", config.PadRight("abc", 6))
}

func TestPadRight_StringAtWidth_Unchanged(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "abc", config.PadRight("abc", 3))
}

func TestPadRight_StringExceedsWidth_Unchanged(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "abcdef", config.PadRight("abcdef", 3))
}

// --- FormatOutput ---

func TestFormatOutput_NoFile_ContainsNoFileMessage(t *testing.T) {
	t.Parallel()
	var sb strings.Builder
	require.NoError(t, config.FormatOutput(&sb, config.DefaultIdxConfig(), "", nil))
	out := sb.String()
	assert.Contains(t, out, "No .idx.yml")
	assert.Contains(t, out, "Tip:")
}

func TestFormatOutput_WithFile_ContainsAllKeys(t *testing.T) {
	t.Parallel()
	var sb strings.Builder
	require.NoError(t, config.FormatOutput(&sb, config.DefaultIdxConfig(), "/project/.idx.yml", nil))
	out := sb.String()
	for _, key := range config.AllKeys() {
		assert.Contains(t, out, key, "expected key %q in output", key)
	}
}

func TestFormatOutput_WithFile_ContainsFilePath(t *testing.T) {
	t.Parallel()
	var sb strings.Builder
	require.NoError(t, config.FormatOutput(&sb, config.DefaultIdxConfig(), "/project/.idx.yml", nil))
	assert.Contains(t, sb.String(), "/project/.idx.yml")
}

func TestFormatOutput_OverriddenKey_ShowsSourceMarker(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultIdxConfig()
	cfg.Search.Format = "json"
	var sb strings.Builder
	require.NoError(t, config.FormatOutput(&sb, cfg, "/project/.idx.yml", []string{config.KeySearchFormat}))
	out := sb.String()
	assert.Contains(t, out, "← .idx.yml")
	assert.Contains(t, out, "(default: text)")
}

func TestFormatOutput_DefaultKey_ShowsDefaultMarker(t *testing.T) {
	t.Parallel()
	var sb strings.Builder
	require.NoError(t, config.FormatOutput(&sb, config.DefaultIdxConfig(), "/project/.idx.yml", nil))
	assert.Contains(t, sb.String(), "· default")
}

func TestFormatOutput_Duration_FormatsCorrectly(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultIdxConfig()
	cfg.Watch.Debounce = 250 * time.Millisecond
	var sb strings.Builder
	require.NoError(t, config.FormatOutput(&sb, cfg, "/project/.idx.yml", []string{config.KeyWatchDebounce}))
	assert.Contains(t, sb.String(), "250ms")
}

func TestFormatOutput_PopularityWeightNotShown(t *testing.T) {
	t.Parallel()
	var sb strings.Builder
	require.NoError(t, config.FormatOutput(&sb, config.DefaultIdxConfig(), "/project/.idx.yml", nil))
	assert.NotContains(t, sb.String(), "popularity_weight")
}

func TestFormatOutput_ReturnsNoError(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultIdxConfig()
	cfg.Search.Format = "json"
	cfg.BM25.K1 = 1.2
	var sb strings.Builder
	err := config.FormatOutput(&sb, cfg, "/p/.idx.yml", []string{config.KeySearchFormat, config.KeyBM25K1})
	require.NoError(t, err)
	assert.NotEmpty(t, sb.String())
}
