package search

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- normalizedSearchOptions ---

func TestNormalizedSearchOptions_NormalizesBehavior(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input Options
		check func(t *testing.T, got Options)
	}{
		{
			name:  "DefaultFormat_ReturnsText",
			input: Options{},
			check: func(t *testing.T, got Options) {
				assert.Equal(t, OutputText, got.Format)
			},
		},
		{
			name:  "NegativeContext_ClampsToZero",
			input: Options{Context: -5},
			check: func(t *testing.T, got Options) {
				assert.Equal(t, 0, got.Context)
			},
		},
		{
			name:  "NegativeFrom_ClampsToZero",
			input: Options{From: -3},
			check: func(t *testing.T, got Options) {
				assert.Equal(t, 0, got.From)
			},
		},
		{
			name:  "NegativeSize_ClampsToZero",
			input: Options{Size: -1},
			check: func(t *testing.T, got Options) {
				assert.Equal(t, 0, got.Size)
			},
		},
		{
			name:  "PrettyJSONWithTextFormat_ClearedToFalse",
			input: Options{Format: OutputText, PrettyJSON: true},
			check: func(t *testing.T, got Options) {
				assert.False(t, got.PrettyJSON)
			},
		},
		{
			name:  "PrettyJSONWithJSONFormat_Preserved",
			input: Options{Format: OutputJSON, PrettyJSON: true},
			check: func(t *testing.T, got Options) {
				assert.True(t, got.PrettyJSON)
			},
		},
		{
			name:  "DefaultOperator_ReturnsAND",
			input: Options{},
			check: func(t *testing.T, got Options) {
				assert.Equal(t, OperatorAND, got.Operator)
			},
		},
		{
			name:  "NegativeRelaxationMin_ClampsToZero",
			input: Options{RelaxationMinExclusive: -2},
			check: func(t *testing.T, got Options) {
				assert.Equal(t, 0, got.RelaxationMinExclusive)
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := normalizedSearchOptions(tc.input)

			// Assert
			tc.check(t, got)
		})
	}
}

// --- normalizeExtensionValue ---

func TestNormalizeExtensionValue_Normalizations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "StripLeadingDot", input: ".go", want: "go"},
		{name: "Lowercase", input: "GO", want: "go"},
		{name: "EmptyReturnsEmpty", input: "  ", want: ""},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, normalizeExtensionValue(tc.input))
		})
	}
}

// --- normalizedExtensionQueries ---

func TestNormalizedExtensionQueries_Deduplicates(t *testing.T) {
	t.Parallel()

	got := normalizedExtensionQueries([]string{"go", ".go", "GO"}, "")
	require.Len(t, got, 1)
	assert.Equal(t, "go", got[0])
}

func TestNormalizedExtensionQueries_SkipsEmpty(t *testing.T) {
	t.Parallel()

	assert.Empty(t, normalizedExtensionQueries([]string{"", "  "}, ""))
}

func TestNormalizedExtensionQueries_UsesFallback(t *testing.T) {
	t.Parallel()

	got := normalizedExtensionQueries(nil, "ts")
	require.Len(t, got, 1)
	assert.Equal(t, "ts", got[0])
}

// --- normalizedFilterQueries ---

func TestNormalizedFilterQueries_Deduplicates(t *testing.T) {
	t.Parallel()

	got := normalizedFilterQueries([]string{"a", "a", "b"}, "")
	assert.Len(t, got, 2)
}

func TestNormalizedFilterQueries_TrimsSpace(t *testing.T) {
	t.Parallel()

	got := normalizedFilterQueries([]string{"  a  "}, "")
	require.Len(t, got, 1)
	assert.Equal(t, "a", got[0])
}

func TestNormalizedFilterQueries_SkipsEmptyStrings(t *testing.T) {
	t.Parallel()

	assert.Empty(t, normalizedFilterQueries([]string{" ", ""}, ""))
}

func TestNormalizedFilterQueries_UsesFallbackWhenEmpty(t *testing.T) {
	t.Parallel()

	got := normalizedFilterQueries(nil, "fallback")
	require.Len(t, got, 1)
	assert.Equal(t, "fallback", got[0])
}

// --- paginatedResults ---

func TestPaginatedResults_FromBeyondLength_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	results := []searchResult{{directoryPath: "/a", fileName: "a.go"}}
	assert.Empty(t, paginatedResults(results, 10, 5))
}

func TestPaginatedResults_NegativeFrom_ClampsToZero(t *testing.T) {
	t.Parallel()

	results := []searchResult{{directoryPath: "/a", fileName: "a.go"}}
	assert.Len(t, paginatedResults(results, -1, 0), 1)
}

func TestPaginatedResults_LimitsResults(t *testing.T) {
	t.Parallel()

	results := []searchResult{
		{directoryPath: "/a", fileName: "a.go"},
		{directoryPath: "/b", fileName: "b.go"},
		{directoryPath: "/c", fileName: "c.go"},
	}
	assert.Len(t, paginatedResults(results, 0, 2), 2)
}

func TestPaginatedResults_ZeroSize_ReturnsAll(t *testing.T) {
	t.Parallel()

	results := []searchResult{
		{directoryPath: "/a", fileName: "a.go"},
		{directoryPath: "/b", fileName: "b.go"},
	}
	assert.Len(t, paginatedResults(results, 0, 0), 2)
}

// --- filesOnlyResult ---

func TestFilesOnlyResult_PreservesFieldsAndClearsMatchedLines(t *testing.T) {
	t.Parallel()

	// Arrange
	input := searchResult{
		directoryPath: "/project/internal",
		fileName:      "service.go",
		score:         1.5,
		matchedLines:  []matchedLine{{lineNumber: 10, content: "func Foo()"}},
	}

	// Act
	got := filesOnlyResult(input)

	// Assert
	assert.Equal(t, input.directoryPath, got.directoryPath)
	assert.Equal(t, input.fileName, got.fileName)
	assert.Equal(t, input.score, got.score)
	assert.Empty(t, got.matchedLines)
}

// --- filesOnlyResults ---

func TestFilesOnlyResults_DeduplicatesSameFileDifferentScores(t *testing.T) {
	t.Parallel()

	// Arrange
	results := []searchResult{
		{directoryPath: "/a", fileName: "foo.go", score: 1.0, matchedLines: []matchedLine{{lineNumber: 1}}},
		{directoryPath: "/a", fileName: "foo.go", score: 2.0, matchedLines: []matchedLine{{lineNumber: 2}}},
	}

	// Act
	got := filesOnlyResults(results)

	// Assert
	require.Len(t, got, 1)
	assert.Equal(t, 2.0, got[0].score, "expected higher score to win")
}

func TestFilesOnlyResults_RemovesMatchedLines(t *testing.T) {
	t.Parallel()

	// Arrange
	results := []searchResult{{directoryPath: "/a", fileName: "bar.go", score: 1.0, matchedLines: []matchedLine{{lineNumber: 5}}}}

	// Act
	got := filesOnlyResults(results)

	// Assert
	require.Len(t, got, 1)
	assert.Empty(t, got[0].matchedLines)
}

func TestFilesOnlyResults_LowerScoreIsNotUpgraded(t *testing.T) {
	t.Parallel()

	// Arrange
	results := []searchResult{
		{directoryPath: "/a", fileName: "foo.go", score: 3.0},
		{directoryPath: "/a", fileName: "foo.go", score: 1.0},
	}

	// Act
	got := filesOnlyResults(results)

	// Assert
	require.Len(t, got, 1)
	assert.Equal(t, 3.0, got[0].score)
}

// --- encodeOutputJSON ---

func TestEncodeOutputJSON_Compact_NoNewlines(t *testing.T) {
	t.Parallel()

	// Arrange
	payload := map[string]any{"count": 0, "results": []any{}}

	// Act
	got, err := encodeOutputJSON(payload, false)

	// Assert
	require.NoError(t, err)
	assert.False(t, strings.Contains(string(got), "\n"), "expected compact JSON (no newlines)")
}

func TestEncodeOutputJSON_Pretty_ContainsNewlines(t *testing.T) {
	t.Parallel()

	// Arrange
	payload := map[string]any{"count": 0, "results": []any{}}

	// Act
	got, err := encodeOutputJSON(payload, true)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, string(got), "\n")
}
