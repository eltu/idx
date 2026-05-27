package search

import (
	"strings"
	"testing"
)

// --- normalizedSearchOptions ---

func TestNormalizedSearchOptionsDefaultFormat(t *testing.T) {
	opts := normalizedSearchOptions(Options{})
	if opts.Format != OutputText {
		t.Errorf("expected default format %q, got %q", OutputText, opts.Format)
	}
}

func TestNormalizedSearchOptionsNegativeContextClampsToZero(t *testing.T) {
	opts := normalizedSearchOptions(Options{Context: -5})
	if opts.Context != 0 {
		t.Errorf("expected context 0, got %d", opts.Context)
	}
}

func TestNormalizedSearchOptionsNegativeFromClampsToZero(t *testing.T) {
	opts := normalizedSearchOptions(Options{From: -3})
	if opts.From != 0 {
		t.Errorf("expected from 0, got %d", opts.From)
	}
}

func TestNormalizedSearchOptionsNegativeSizeClampsToZero(t *testing.T) {
	opts := normalizedSearchOptions(Options{Size: -1})
	if opts.Size != 0 {
		t.Errorf("expected size 0, got %d", opts.Size)
	}
}

func TestNormalizedSearchOptionsClearsPrettyJSONForTextFormat(t *testing.T) {
	opts := normalizedSearchOptions(Options{Format: OutputText, PrettyJSON: true})
	if opts.PrettyJSON {
		t.Error("expected PrettyJSON false for text format")
	}
}

func TestNormalizedSearchOptionsKeepsPrettyJSONForJSONFormat(t *testing.T) {
	opts := normalizedSearchOptions(Options{Format: OutputJSON, PrettyJSON: true})
	if !opts.PrettyJSON {
		t.Error("expected PrettyJSON true for JSON format")
	}
}

func TestNormalizedSearchOptionsDefaultOperator(t *testing.T) {
	opts := normalizedSearchOptions(Options{})
	if opts.Operator != OperatorAND {
		t.Errorf("expected AND operator, got %q", opts.Operator)
	}
}

func TestNormalizedSearchOptionsNegativeRelaxationMinClampsToZero(t *testing.T) {
	opts := normalizedSearchOptions(Options{RelaxationMinExclusive: -2})
	if opts.RelaxationMinExclusive != 0 {
		t.Errorf("expected 0, got %d", opts.RelaxationMinExclusive)
	}
}

// --- normalizeExtensionValue ---

func TestNormalizeExtensionValueStripsLeadingDot(t *testing.T) {
	got := normalizeExtensionValue(".go")
	if got != "go" {
		t.Errorf("expected %q, got %q", "go", got)
	}
}

func TestNormalizeExtensionValueLowercases(t *testing.T) {
	got := normalizeExtensionValue("GO")
	if got != "go" {
		t.Errorf("expected %q, got %q", "go", got)
	}
}

func TestNormalizeExtensionValueEmptyReturnsEmpty(t *testing.T) {
	got := normalizeExtensionValue("  ")
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

// --- normalizedExtensionQueries ---

func TestNormalizedExtensionQueriesDeduplicates(t *testing.T) {
	got := normalizedExtensionQueries([]string{"go", ".go", "GO"}, "")
	if len(got) != 1 || got[0] != "go" {
		t.Errorf("expected [go], got %v", got)
	}
}

func TestNormalizedExtensionQueriesSkipsEmpty(t *testing.T) {
	got := normalizedExtensionQueries([]string{"", "  "}, "")
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}

func TestNormalizedExtensionQueriesUsesFallback(t *testing.T) {
	got := normalizedExtensionQueries(nil, "ts")
	if len(got) != 1 || got[0] != "ts" {
		t.Errorf("expected [ts], got %v", got)
	}
}

// --- normalizedFilterQueries ---

func TestNormalizedFilterQueriesDeduplicates(t *testing.T) {
	got := normalizedFilterQueries([]string{"a", "a", "b"}, "")
	if len(got) != 2 {
		t.Errorf("expected 2 unique queries, got %v", got)
	}
}

func TestNormalizedFilterQueriesTrimsSpace(t *testing.T) {
	got := normalizedFilterQueries([]string{"  a  "}, "")
	if len(got) != 1 || got[0] != "a" {
		t.Errorf("expected [a], got %v", got)
	}
}

func TestNormalizedFilterQueriesSkipsEmptyStrings(t *testing.T) {
	got := normalizedFilterQueries([]string{" ", ""}, "")
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestNormalizedFilterQueriesUsesFallbackWhenEmpty(t *testing.T) {
	got := normalizedFilterQueries(nil, "fallback")
	if len(got) != 1 || got[0] != "fallback" {
		t.Errorf("expected [fallback], got %v", got)
	}
}

// --- paginatedResults ---

func TestPaginatedResultsFromBeyondLengthReturnsEmpty(t *testing.T) {
	results := []searchResult{{directoryPath: "/a", fileName: "a.go"}}
	got := paginatedResults(results, 10, 5)
	if len(got) != 0 {
		t.Errorf("expected empty, got %d results", len(got))
	}
}

func TestPaginatedResultsNegativeFromClampsToZero(t *testing.T) {
	results := []searchResult{{directoryPath: "/a", fileName: "a.go"}}
	got := paginatedResults(results, -1, 0)
	if len(got) != 1 {
		t.Errorf("expected 1 result, got %d", len(got))
	}
}

func TestPaginatedResultsLimitsResults(t *testing.T) {
	results := []searchResult{
		{directoryPath: "/a", fileName: "a.go"},
		{directoryPath: "/b", fileName: "b.go"},
		{directoryPath: "/c", fileName: "c.go"},
	}
	got := paginatedResults(results, 0, 2)
	if len(got) != 2 {
		t.Errorf("expected 2, got %d", len(got))
	}
}

func TestPaginatedResultsZeroSizeReturnsAll(t *testing.T) {
	results := []searchResult{
		{directoryPath: "/a", fileName: "a.go"},
		{directoryPath: "/b", fileName: "b.go"},
	}
	got := paginatedResults(results, 0, 0)
	if len(got) != 2 {
		t.Errorf("expected 2, got %d", len(got))
	}
}

// --- filesOnlyResults ---

func TestFilesOnlyResultsDeduplicatesSameFileDifferentScores(t *testing.T) {
	results := []searchResult{
		{directoryPath: "/a", fileName: "foo.go", score: 1.0, matchedLines: []matchedLine{{lineNumber: 1}}},
		{directoryPath: "/a", fileName: "foo.go", score: 2.0, matchedLines: []matchedLine{{lineNumber: 2}}},
	}
	got := filesOnlyResults(results)
	if len(got) != 1 {
		t.Fatalf("expected 1 unique file, got %d", len(got))
	}
	if got[0].score != 2.0 {
		t.Errorf("expected higher score 2.0 to win, got %v", got[0].score)
	}
}

func TestFilesOnlyResultsRemovesMatchedLines(t *testing.T) {
	results := []searchResult{
		{directoryPath: "/a", fileName: "bar.go", score: 1.0, matchedLines: []matchedLine{{lineNumber: 5}}},
	}
	got := filesOnlyResults(results)
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got))
	}
	if len(got[0].matchedLines) != 0 {
		t.Error("expected matched lines to be stripped in files-only mode")
	}
}

func TestFilesOnlyResultsLowerScoreIsNotUpgraded(t *testing.T) {
	results := []searchResult{
		{directoryPath: "/a", fileName: "foo.go", score: 3.0},
		{directoryPath: "/a", fileName: "foo.go", score: 1.0},
	}
	got := filesOnlyResults(results)
	if len(got) != 1 || got[0].score != 3.0 {
		t.Errorf("expected score 3.0 to be kept, got %v", got)
	}
}

// --- matchesOnlyResults ---

func TestMatchesOnlyResultsDropsResultsWithNoMatchedLines(t *testing.T) {
	results := []searchResult{
		{directoryPath: "/a", fileName: "a.go", matchedLines: []matchedLine{
			{lineNumber: 1, isMatch: false},
		}},
	}
	got := matchesOnlyResults(results)
	if len(got) != 0 {
		t.Errorf("expected empty (no actual matches), got %d", len(got))
	}
}

func TestMatchesOnlyResultsKeepsStaleEntries(t *testing.T) {
	results := []searchResult{
		{directoryPath: "/a", fileName: "stale.go", stale: true},
	}
	got := matchesOnlyResults(results)
	if len(got) != 1 {
		t.Errorf("expected stale entry to be kept, got %d", len(got))
	}
}

func TestMatchesOnlyResultsFiltersNonMatchLines(t *testing.T) {
	results := []searchResult{
		{directoryPath: "/a", fileName: "a.go", matchedLines: []matchedLine{
			{lineNumber: 1, isMatch: false},
			{lineNumber: 2, isMatch: true},
		}},
	}
	got := matchesOnlyResults(results)
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got))
	}
	if len(got[0].matchedLines) != 1 || !got[0].matchedLines[0].isMatch {
		t.Errorf("expected only matched line retained, got %+v", got[0].matchedLines)
	}
}

// --- encodeOutputJSON ---

func TestEncodeOutputJSONCompact(t *testing.T) {
	payload := map[string]any{"count": 0, "results": []any{}}
	got, err := encodeOutputJSON(payload, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(string(got), "\n") {
		t.Error("expected compact JSON (no newlines)")
	}
}

func TestEncodeOutputJSONPretty(t *testing.T) {
	payload := map[string]any{"count": 0, "results": []any{}}
	got, err := encodeOutputJSON(payload, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(string(got), "\n") {
		t.Error("expected indented JSON (with newlines)")
	}
}
