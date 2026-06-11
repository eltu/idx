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

// writeIdxYml writes an .idx.yml file to the given project root.
func writeIdxYml(t *testing.T, projectRoot, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, ".idx.yml"), []byte(content), 0o644))
}

// runConfigShow executes `idx config show` with the server running and returns the captured output.
func runConfigShow(t *testing.T) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	err := run([]string{"idx", "config", "show"}, &buf)
	return buf.String(), err
}

// --- Group A: idx config show output (server required) ---

func TestCLI_ConfigShow_WithoutFile_OutputsNoFileMessage(t *testing.T) {
	// Arrange — project with no .idx.yml
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)

	// Act
	out, err := runConfigShow(t)

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
	startTestServer(t, projectRoot)

	// Act
	out, err := runConfigShow(t)

	// Assert
	require.NoError(t, err)
	expectedKeys := []string{
		"search.format", "search.limit", "search.operator", "search.context",
		"search.relaxation", "search.cache_ttl", "search.max_workers",
		"watch.debounce", "index.ignore",
		"bm25.k1", "bm25.b", "bm25.proximity_weight",
		"log.level",
	}
	for _, key := range expectedKeys {
		assert.Contains(t, out, key, "expected key %q in config show output", key)
	}
}

func TestCLI_ConfigShow_WithFile_MarksOverriddenKeyWithSourceLabel(t *testing.T) {
	// Arrange — one key set in file
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	writeIdxYml(t, projectRoot, "search:\n  format: json\n")
	startTestServer(t, projectRoot)

	// Act
	out, err := runConfigShow(t)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, out, "← .idx.yml", "expected override marker for key set in file")
	assert.Contains(t, out, "· default", "expected default marker for keys not in file")
}

func TestCLI_ConfigShow_WithMultipleOverrides_CountInPath(t *testing.T) {
	// Arrange — two overrides
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	writeIdxYml(t, projectRoot, "search:\n  format: json\n  size: 5\n")
	startTestServer(t, projectRoot)

	// Act
	out, err := runConfigShow(t)

	// Assert
	require.NoError(t, err)
	overrideCount := strings.Count(out, "← .idx.yml")
	assert.Equal(t, 2, overrideCount, "expected exactly 2 keys marked as overridden")
}

func TestCLI_ConfigShow_PopularityWeightDisplayed(t *testing.T) {
	// Arrange — popularity_weight is a configurable key and must appear in config show
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	writeIdxYml(t, projectRoot, "bm25:\n  popularity_weight: 0.9\n")
	startTestServer(t, projectRoot)

	// Act
	out, err := runConfigShow(t)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, out, "bm25.popularity_weight",
		"bm25.popularity_weight must appear in config show output")
	assert.Contains(t, out, "0.9",
		"overridden popularity_weight value must appear in output")
}

func TestCLI_ConfigShow_WithoutServer_ReturnsError(t *testing.T) {
	// Arrange — no server; IDX_PROJECT_PATH points to a temp dir
	root, err := os.MkdirTemp("/tmp", "idx-e2e-cfg-nosrv")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	t.Setenv("IDX_PROJECT_PATH", root)

	// Act
	_, err = runConfigShow(t)

	// Assert
	require.Error(t, err)
	assert.ErrorContains(t, err, "server not running")
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

// --- New UX: config get subcommand ---

func TestCLI_Config_Get_KnownKey_Succeeds(t *testing.T) {
	// Arrange — config get is a local command; no server required
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)

	// Act
	// cmd.OutOrStdout() goes to os.Stdout in the E2E context (not the io.Discard writer),
	// so we verify correct behavior via the error return, not buffer capture.
	// Value correctness is covered by unit tests for runConfigGetTo.
	err := run([]string{"idx", "config", "get", "search.operator"}, io.Discard)

	// Assert — known key succeeds without error
	require.NoError(t, err)
}

func TestCLI_Config_Get_UnknownKey_ReturnsError(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)

	// Act
	err := run([]string{"idx", "config", "get", "nonexistent.key"}, io.Discard)

	// Assert
	require.Error(t, err)
	assert.ErrorContains(t, err, "unknown config key")
}
