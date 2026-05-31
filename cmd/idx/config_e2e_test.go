package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureConfigOutput redirects os.Stdout to a pipe while calling f, then
// returns everything written to stdout. Required because config_commands.go
// writes directly via fmt.Printf rather than through the output writer.
// Must not be used with t.Parallel — it mutates the global os.Stdout.
func captureConfigOutput(t *testing.T, f func() error) (string, error) {
	t.Helper()
	saved := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	runErr := f()

	w.Close()
	os.Stdout = saved

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	return buf.String(), runErr
}

// writeIdxYml writes an .idx.yml file to the given project root.
func writeIdxYml(t *testing.T, projectRoot, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, ".idx.yml"), []byte(content), 0o644))
}

// --- Group A: idx config show output ---

func TestCLI_ConfigShow_WithoutFile_OutputsNoFileMessage(t *testing.T) {
	// Arrange — project with no .idx.yml
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)

	// Act
	out, err := captureConfigOutput(t, func() error {
		return run([]string{"idx", "config", "show"}, io.Discard)
	})

	// Assert
	require.NoError(t, err)
	assert.Contains(t, out, "No .idx.yml")
	assert.Contains(t, out, "Tip:")
}

func TestCLI_ConfigShow_WithFile_ShowsAllThirteenKeys(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	writeIdxYml(t, projectRoot, "search:\n  format: json\n")

	// Act
	out, err := captureConfigOutput(t, func() error {
		return run([]string{"idx", "config", "show"}, io.Discard)
	})

	// Assert
	require.NoError(t, err)
	expectedKeys := []string{
		"search.format",
		"search.size",
		"search.operator",
		"search.context",
		"search.relaxation",
		"search.cache_ttl",
		"search.max_workers",
		"watch.debounce",
		"index.ignore",
		"bm25.k1",
		"bm25.b",
		"bm25.proximity_weight",
		"log.level",
	}
	for _, key := range expectedKeys {
		assert.Contains(t, out, key, "expected key %q in config show output", key)
	}
}

func TestCLI_ConfigShow_WithFile_MarksOverriddenKeyWithSourceLabel(t *testing.T) {
	// Arrange — set one key so it appears as overridden
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	writeIdxYml(t, projectRoot, "search:\n  format: json\n")

	// Act
	out, err := captureConfigOutput(t, func() error {
		return run([]string{"idx", "config", "show"}, io.Discard)
	})

	// Assert
	require.NoError(t, err)
	// The overridden key shows its current value and the source marker.
	// Keys at default show "· default".
	assert.Contains(t, out, "← .idx.yml", "expected override marker for key set in file")
	assert.Contains(t, out, "· default", "expected default marker for keys not in file")
}

func TestCLI_ConfigShow_WithMultipleOverrides_CountInPath(t *testing.T) {
	// Arrange — two overrides
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	writeIdxYml(t, projectRoot, "search:\n  format: json\n  size: 5\n")

	// Act — run idx status (which shows the banner) to see override count
	out, err := captureConfigOutput(t, func() error {
		return run([]string{"idx", "config", "show"}, io.Discard)
	})

	// Assert
	require.NoError(t, err)
	// Both overridden keys must show the source label
	overrideCount := strings.Count(out, "← .idx.yml")
	assert.Equal(t, 2, overrideCount, "expected exactly 2 keys marked as overridden")
}

func TestCLI_ConfigShow_PopularityWeightNotDisplayed(t *testing.T) {
	// Arrange — popularity_weight is valid in .idx.yml but intentionally absent from
	// config show (it is not a member of the 13 displayed keys).
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	writeIdxYml(t, projectRoot, "bm25:\n  popularity_weight: 0.9\n")

	// Act
	out, err := captureConfigOutput(t, func() error {
		return run([]string{"idx", "config", "show"}, io.Discard)
	})

	// Assert
	require.NoError(t, err)
	assert.NotContains(t, out, "popularity_weight",
		"bm25.popularity_weight must not appear in config show output")
}

// --- Group B: config values flow through to search behavior ---

func TestCLI_Config_SearchOperatorFromFile_AffectsSearchDefault(t *testing.T) {
	// Arrange — .idx.yml sets operator: OR so multi-term search returns results
	// even when terms are in different files (which AND would miss).
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	writeIdxYml(t, projectRoot, "search:\n  operator: OR\n")
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Act — "Logger" is in internal/logger.go; "TokenHandler" is in internal/handler.go.
	// With OR as default, both are returned without an explicit --operator flag.
	out, err := runSearch(t, "Logger", "TokenHandler", "--format", "json")

	// Assert
	require.NoError(t, err)
	resp := parseSearchJSON(t, out)
	assert.GreaterOrEqual(t, resp.Count, 2,
		"expected OR default from .idx.yml to return results from both files")
}

func TestCLI_Config_SearchSizeFromFile_LimitsResults(t *testing.T) {
	// Arrange — .idx.yml sets size: 1 so searches return at most 1 result by default.
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	writeIdxYml(t, projectRoot, "search:\n  size: 1\n")
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Act — "token" matches multiple files; size from config caps the result set.
	out, err := runSearch(t, "token", "--operator", "OR", "--format", "json")

	// Assert
	require.NoError(t, err)
	resp := parseSearchJSON(t, out)
	assert.Equal(t, 1, len(resp.Results),
		"expected size: 1 from .idx.yml to cap search results")
}

func TestCLI_Config_PopularityWeightFromFile_SearchSucceeds(t *testing.T) {
	// Arrange — popularity_weight is loaded from .idx.yml even though it is not
	// shown in config show. The value flows to search ranking via the SearchRequest.
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	writeIdxYml(t, projectRoot, "bm25:\n  popularity_weight: 0.9\n")
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Act — no --popularity-weight flag; the file default applies
	out, err := runSearch(t, "BM25Tokenizer", "--format", "json")

	// Assert
	require.NoError(t, err)
	resp := parseSearchJSON(t, out)
	assert.GreaterOrEqual(t, resp.Count, 1,
		"expected search to succeed with popularity_weight set via .idx.yml")
}
