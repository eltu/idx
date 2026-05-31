package main

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	idxipc "idx/internal/shared/ipc"
)

// runSearch executes `idx search <args>` against the already-running server and
// returns the captured stdout and any error.
// Example: out, err := runSearch(t, "BM25Tokenizer", "--format", "json").
func runSearch(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	err := run(append([]string{"idx", "search"}, args...), &buf)
	return buf.String(), err
}

// parseSearchJSON unmarshals a standard search JSON response.
func parseSearchJSON(t *testing.T, output string) idxipc.SearchResponse {
	t.Helper()
	var resp idxipc.SearchResponse
	require.NoError(t, json.Unmarshal([]byte(output), &resp), "output is not valid SearchResponse JSON: %s", output)
	return resp
}

// parseFilesOnlyJSON unmarshals a files-only JSON response (array of strings).
func parseFilesOnlyJSON(t *testing.T, output string) []string {
	t.Helper()
	var paths []string
	require.NoError(t, json.Unmarshal([]byte(output), &paths), "output is not valid files-only JSON: %s", output)
	return paths
}

// --- Group A: client-side validation (no server needed) ---

func TestCLI_Search_Validation_RejectsInvalidFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		args        []string
		errContains string
	}{
		{
			name:        "missing_query",
			args:        []string{},
			errContains: "",
		},
		{
			name:        "invalid_format",
			args:        []string{"token", "--format", "xml"},
			errContains: "unsupported --format",
		},
		{
			name:        "json_pretty_without_json_format",
			args:        []string{"token", "--json-pretty"},
			errContains: "--json-pretty requires --format json",
		},
		{
			name:        "invalid_operator",
			args:        []string{"token", "--operator", "XOR"},
			errContains: "unsupported --operator",
		},
		{
			name:        "negative_context",
			args:        []string{"token", "--context", "-1"},
			errContains: "invalid --context",
		},
		{
			name:        "negative_from",
			args:        []string{"token", "--from", "-1"},
			errContains: "invalid --from",
		},
		{
			name:        "zero_size_explicitly_set",
			args:        []string{"token", "--size", "0"},
			errContains: "invalid --size",
		},
		{
			name:        "relaxation_with_operator_OR",
			args:        []string{"token", "--operator", "OR", "--relaxation", ">1"},
			errContains: "invalid --relaxation with --operator",
		},
		{
			name:        "relaxation_format_missing_gt",
			args:        []string{"token", "--relaxation", "2"},
			errContains: "expected format >N",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var buf bytes.Buffer

			// Act
			err := run(append([]string{"idx", "search"}, tc.args...), &buf)

			// Assert
			require.Error(t, err)
			if tc.errContains != "" {
				assert.ErrorContains(t, err, tc.errContains)
			}
		})
	}
}

// --- Group B: output format flags (server required) ---

func TestCLI_Search_TextFormat_ContainsFilePath(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Act
	out, err := runSearch(t, "BM25Tokenizer")

	// Assert
	require.NoError(t, err)
	assert.Contains(t, out, "tokenizer.go")
	assert.False(t, strings.HasPrefix(strings.TrimSpace(out), "{"), "expected text format, got JSON-like output")
}

func TestCLI_Search_JSONFormat_IsValidJSON(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Act
	out, err := runSearch(t, "BM25Tokenizer", "--format", "json")

	// Assert
	require.NoError(t, err)
	resp := parseSearchJSON(t, out)
	assert.GreaterOrEqual(t, resp.Count, 1)
	require.NotEmpty(t, resp.Results)
	assert.Contains(t, resp.Results[0].File, "tokenizer")
}

func TestCLI_Search_JSONPretty_HasIndentation(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Act
	out, err := runSearch(t, "BM25Tokenizer", "--format", "json", "--json-pretty")

	// Assert
	require.NoError(t, err)
	assert.Contains(t, out, "\n  ", "expected pretty-printed JSON with indentation")
	resp := parseSearchJSON(t, out)
	assert.GreaterOrEqual(t, resp.Count, 1)
}

func TestCLI_Search_Explain_ScorePresentInJSON(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Act
	out, err := runSearch(t, "BM25Tokenizer", "--format", "json", "--explain")

	// Assert
	require.NoError(t, err)
	resp := parseSearchJSON(t, out)
	require.NotEmpty(t, resp.Results)
	assert.NotNil(t, resp.Results[0].Score, "expected score field when --explain is set")
}

func TestCLI_Search_AgentCompact_NoDoubleBlankLines(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Act
	out, err := runSearch(t, "BM25Tokenizer", "--agent-compact")

	// Assert
	require.NoError(t, err)
	assert.Contains(t, out, "tokenizer.go")
	assert.NotContains(t, out, "\n\n", "agent-compact output should have no blank lines")
}

// --- Group C: content filter flags ---

func TestCLI_Search_MatchesOnly_FewerLinesThanDefault(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Act
	outDefault, err := runSearch(t, "BM25Tokenizer", "--context", "3")
	require.NoError(t, err)
	outFiltered, err := runSearch(t, "BM25Tokenizer", "--context", "3", "--matches-only")
	require.NoError(t, err)

	// Assert — matches-only should suppress context lines, producing shorter output
	assert.LessOrEqual(t, len(outFiltered), len(outDefault))
}

func TestCLI_Search_FilesOnly_ReturnsPathsWithoutLineNumbers(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Act
	out, err := runSearch(t, "BM25Tokenizer", "--files-only")

	// Assert
	require.NoError(t, err)
	assert.Contains(t, out, "tokenizer.go")
	// files-only should not emit "line: content" entries
	assert.NotContains(t, out, ": BM25Tokenizer")
}

func TestCLI_Search_FilesOnly_JSONFormat_ReturnsStringArray(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Act
	out, err := runSearch(t, "BM25Tokenizer", "--files-only", "--format", "json")

	// Assert
	require.NoError(t, err)
	paths := parseFilesOnlyJSON(t, out)
	assert.NotEmpty(t, paths)
	assert.True(t, func() bool {
		for _, p := range paths {
			if strings.Contains(p, "tokenizer") {
				return true
			}
		}
		return false
	}(), "expected tokenizer.go in files-only JSON result")
}

// --- Group D: metadata filter flags ---

func TestCLI_Search_ExtFilter_ReturnsGoFiles(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Act
	out, err := runSearch(t, "BM25Tokenizer", "--ext", "go")

	// Assert
	require.NoError(t, err)
	assert.Contains(t, out, "tokenizer.go")
}

func TestCLI_Search_ExtFilterWithDot_SameBehaviorAsWithout(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Act
	outNoDot, err := runSearch(t, "BM25Tokenizer", "--ext", "go", "--format", "json")
	require.NoError(t, err)
	outDot, err := runSearch(t, "BM25Tokenizer", "--ext", ".go", "--format", "json")
	require.NoError(t, err)

	// Assert — both forms should yield the same count
	respNoDot := parseSearchJSON(t, outNoDot)
	respDot := parseSearchJSON(t, outDot)
	assert.Equal(t, respNoDot.Count, respDot.Count)
}

func TestCLI_Search_PathFilter_RestrictsToPkgDir(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Act
	out, err := runSearch(t, "token", "--path", "pkg", "--format", "json")

	// Assert
	require.NoError(t, err)
	resp := parseSearchJSON(t, out)
	for _, r := range resp.Results {
		assert.Contains(t, r.File, "pkg/", "expected only pkg/ files, got: %s", r.File)
		assert.NotContains(t, r.File, "internal/")
	}
}

func TestCLI_Search_PathFilter_RestrictsToInternalDir(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Act
	out, err := runSearch(t, "token", "--path", "internal", "--format", "json")

	// Assert
	require.NoError(t, err)
	resp := parseSearchJSON(t, out)
	require.NotEmpty(t, resp.Results, "expected results in internal/ for query 'token'")
	for _, r := range resp.Results {
		assert.Contains(t, r.File, "internal/", "expected only internal/ files, got: %s", r.File)
		assert.NotContains(t, r.File, "pkg/")
	}
}

func TestCLI_Search_PathAndExtFilter_Combined(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Act
	out, err := runSearch(t, "token", "--path", "pkg", "--ext", "go", "--format", "json")

	// Assert
	require.NoError(t, err)
	resp := parseSearchJSON(t, out)
	for _, r := range resp.Results {
		assert.Contains(t, r.File, "pkg/")
		assert.NotContains(t, r.File, "internal/")
	}
}

func TestCLI_Search_ExtOnly_NoQuery_ReturnsResults(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Act — no query term: ext alone is a valid search
	out, err := runSearch(t, "--ext", "go", "--format", "json")

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, out)
	resp := parseSearchJSON(t, out)
	assert.GreaterOrEqual(t, resp.Count, 1)
}

func TestCLI_Search_PathOnly_NoQuery_ReturnsResults(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Act — no query term: path alone is a valid search
	out, err := runSearch(t, "--path", "pkg", "--format", "json")

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, out)
	resp := parseSearchJSON(t, out)
	assert.GreaterOrEqual(t, resp.Count, 1)
}

// --- Group E: boolean operator ---

func TestCLI_Search_OperatorAND_FindsFileWithBothTerms(t *testing.T) {
	// Arrange — "BM25" and "pipeline" co-occur only in internal/handler.go
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Act
	out, err := runSearch(t, "BM25", "pipeline", "--operator", "AND", "--format", "json")

	// Assert
	require.NoError(t, err)
	resp := parseSearchJSON(t, out)
	require.NotEmpty(t, resp.Results, "expected at least one result for 'BM25 pipeline' AND")
	assert.Contains(t, resp.Results[0].File, "handler.go")
}

func TestCLI_Search_OperatorOR_MatchesFilesWithAnyTerm(t *testing.T) {
	// Arrange — "Logger" is in logger.go; "TokenHandler" is in handler.go
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Act
	out, err := runSearch(t, "Logger", "TokenHandler", "--operator", "OR", "--format", "json")

	// Assert
	require.NoError(t, err)
	resp := parseSearchJSON(t, out)
	assert.GreaterOrEqual(t, resp.Count, 2, "expected both logger.go and handler.go")
	files := make([]string, 0, len(resp.Results))
	for _, r := range resp.Results {
		files = append(files, r.File)
	}
	assert.True(t, func() bool {
		for _, f := range files {
			if strings.Contains(f, "logger.go") {
				return true
			}
		}
		return false
	}(), "expected logger.go in OR results")
	assert.True(t, func() bool {
		for _, f := range files {
			if strings.Contains(f, "handler.go") {
				return true
			}
		}
		return false
	}(), "expected handler.go in OR results")
}

func TestCLI_Search_OperatorAND_NoResults_WhenTermsNotColocated(t *testing.T) {
	// Arrange — "hello" is only in main.go; "BM25Tokenizer" is only in tokenizer.go
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Act
	out, err := runSearch(t, "hello", "BM25Tokenizer", "--operator", "AND", "--format", "json")

	// Assert
	require.NoError(t, err)
	resp := parseSearchJSON(t, out)
	assert.Equal(t, 0, resp.Count, "expected no results when AND terms are in different files")
}

// --- Group F: pagination ---

func TestCLI_Search_Size_LimitsResultCount(t *testing.T) {
	// Arrange — "token" matches logger.go and handler.go (>= 2 results)
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Act
	outAll, err := runSearch(t, "token", "--operator", "OR", "--format", "json")
	require.NoError(t, err)
	outOne, err := runSearch(t, "token", "--operator", "OR", "--size", "1", "--format", "json")
	require.NoError(t, err)

	// Assert
	respAll := parseSearchJSON(t, outAll)
	respOne := parseSearchJSON(t, outOne)
	assert.GreaterOrEqual(t, respAll.Count, 2)
	assert.Len(t, respOne.Results, 1)
}

func TestCLI_Search_From_SkipsFirstResult(t *testing.T) {
	// Arrange — query with multiple results
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Act
	outBase, err := runSearch(t, "token", "--operator", "OR", "--format", "json")
	require.NoError(t, err)
	outSkip, err := runSearch(t, "token", "--operator", "OR", "--from", "1", "--format", "json")
	require.NoError(t, err)

	// Assert — first result in skip run should differ from base run's first
	respBase := parseSearchJSON(t, outBase)
	respSkip := parseSearchJSON(t, outSkip)
	require.GreaterOrEqual(t, respBase.Count, 2, "need >= 2 results to test --from")
	require.NotEmpty(t, respSkip.Results)
	assert.NotEqual(t, respBase.Results[0].File, respSkip.Results[0].File)
}

func TestCLI_Search_FromAndSize_ReturnsExactlyOneResult(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Act
	out, err := runSearch(t, "token", "--operator", "OR", "--from", "1", "--size", "1", "--format", "json")

	// Assert
	require.NoError(t, err)
	resp := parseSearchJSON(t, out)
	assert.Len(t, resp.Results, 1)
}

// --- Group G: relaxation ---

func TestCLI_Search_Relaxation_FindsResultsWhenStrictANDWouldFail(t *testing.T) {
	// Arrange — "hello", "BM25", "scoring" span multiple files; strict AND yields 0
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Act — strict AND
	outStrict, err := runSearch(t, "hello", "BM25", "scoring", "--operator", "AND", "--format", "json")
	require.NoError(t, err)

	// Act — relaxed AND (remove trailing terms until a match is found)
	outRelaxed, err := runSearch(t, "hello", "BM25", "scoring", "--operator", "AND", "--relaxation", ">1", "--format", "json")
	require.NoError(t, err)

	// Assert
	respStrict := parseSearchJSON(t, outStrict)
	respRelaxed := parseSearchJSON(t, outRelaxed)
	assert.Equal(t, 0, respStrict.Count, "expected strict AND to return 0 results")
	assert.Greater(t, respRelaxed.Count, 0, "expected relaxed AND to return results")
}

// --- Group H: popularity weight ---

func TestCLI_Search_PopularityWeightZero_Succeeds(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Act
	out, err := runSearch(t, "BM25Tokenizer", "--popularity-weight", "0")

	// Assert
	require.NoError(t, err)
	assert.Contains(t, out, "tokenizer.go")
}

func TestCLI_Search_PopularityWeightHigh_Succeeds(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Act
	out, err := runSearch(t, "BM25Tokenizer", "--popularity-weight", "1.5")

	// Assert
	require.NoError(t, err)
	assert.Contains(t, out, "tokenizer.go")
}

// --- Group I: no results ---

func TestCLI_Search_NoResults_TextFormat_OutputsMessage(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Act
	out, err := runSearch(t, "xyzzynonexistentterm123")

	// Assert
	require.NoError(t, err)
	assert.Contains(t, out, "No results found")
}

func TestCLI_Search_NoResults_JSONFormat_ReturnsZeroCount(t *testing.T) {
	// Arrange
	projectRoot := newTestProject(t)
	t.Chdir(projectRoot)
	startTestServer(t, projectRoot)
	require.NoError(t, run([]string{"idx", "init"}, io.Discard))

	// Act
	out, err := runSearch(t, "xyzzynonexistentterm123", "--format", "json")

	// Assert
	require.NoError(t, err)
	resp := parseSearchJSON(t, out)
	assert.Equal(t, 0, resp.Count)
	assert.Empty(t, resp.Results)
}
