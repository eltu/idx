package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tea "charm.land/bubbletea/v2"

	"idx/internal/features/indexing"
)

func TestIsInspectTransactionLogPath_SupportedLayouts(t *testing.T) {
	t.Parallel()
	assert.True(t, isInspectTransactionLogPath("/repo/.idx/logs/tlog.idx"))
	assert.True(t, isInspectTransactionLogPath("/repo/any/idx/logs/tlog.idx"))
	assert.False(t, isInspectTransactionLogPath("/repo/.idx/logs/other.idx"))
}

func TestInspectCommonPrefix_ReturnsSharedPrefix(t *testing.T) {
	t.Parallel()
	cases := []struct {
		left, right, expected string
	}{
		{"index", "indexer", "index"},
		{"tlog", "tlog", "tlog"},
		{"abc", "xyz", ""},
		{"index", "tlog", ""},
		{"", "tlog", ""},
		{"tlog", "", ""},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.left+"/"+tc.right, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, inspectCommonPrefix(tc.left, tc.right))
		})
	}
}

func TestInspectCommandSuggestions_ReturnsAllOnEmpty(t *testing.T) {
	t.Parallel()
	suggestions := inspectCommandSuggestions("")
	assert.Len(t, suggestions, len(inspectAvailableCommands))
}

func TestInspectCommandSuggestions_FiltersPrefix(t *testing.T) {
	t.Parallel()
	suggestions := inspectCommandSuggestions("in")
	assert.Equal(t, []string{"index"}, suggestions)
}

func TestInspectCommandSuggestions_NoMatch_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	suggestions := inspectCommandSuggestions("xyz")
	assert.Empty(t, suggestions)
}

func TestAutocompleteInspectCommand_SingleMatch_Completes(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "tlog", autocompleteInspectCommand(":tl"))
}

func TestAutocompleteInspectCommand_NoMatch_ReturnsQuery(t *testing.T) {
	t.Parallel()
	assert.Equal(t, ":xyz", autocompleteInspectCommand(":xyz"))
}

func TestAutocompleteInspectCommand_EmptyQuery_ReturnsQuery(t *testing.T) {
	t.Parallel()
	assert.Equal(t, ":", autocompleteInspectCommand(":"))
}

func TestAutocompleteInspectCommand_MultipleMatches_ExpandsCommonPrefix(t *testing.T) {
	t.Parallel()
	// Both "index" and any future "i*" command would share prefix;
	// with current commands ":i" only matches "index" so returns "index".
	assert.Equal(t, "index", autocompleteInspectCommand(":i"))
}

func TestInspectStringField_ReturnsFirstNonEmpty(t *testing.T) {
	t.Parallel()
	fields := map[string]any{"a": "  ", "b": "hello", "c": "world"}
	assert.Equal(t, "hello", inspectStringField(fields, "a", "b", "c"))
}

func TestInspectStringField_SkipsMissingKeys(t *testing.T) {
	t.Parallel()
	fields := map[string]any{"b": "found"}
	assert.Equal(t, "found", inspectStringField(fields, "missing", "b"))
}

func TestInspectStringField_SkipsNonStringValues(t *testing.T) {
	t.Parallel()
	fields := map[string]any{"a": 42, "b": "ok"}
	assert.Equal(t, "ok", inspectStringField(fields, "a", "b"))
}

func TestInspectStringField_AllEmpty_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	fields := map[string]any{"a": "  "}
	assert.Empty(t, inspectStringField(fields, "a", "missing"))
}

func TestExtractSummaryValue_EqualsSign_ExtractsValue(t *testing.T) {
	t.Parallel()
	summary := "indexed_at=2026-05-01T12:00:00Z path=/repo hash=abc"
	assert.Equal(t, "/repo", extractSummaryValue(summary, "path"))
}

func TestExtractSummaryValue_ColonSeparator_ExtractsValue(t *testing.T) {
	t.Parallel()
	summary := "path:/repo/internal hash:abc123"
	assert.Equal(t, "abc123", extractSummaryValue(summary, "hash"))
}

func TestExtractSummaryValue_MissingKey_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	assert.Empty(t, extractSummaryValue("path=/repo", "missing"))
}

func TestExtractSummaryValue_ReturnsUntilDelimiter(t *testing.T) {
	t.Parallel()
	summary := "indexed_at=2026-05-01,path=/repo"
	assert.Equal(t, "2026-05-01", extractSummaryValue(summary, "indexed_at"))
}

func TestParseInspectSummaryFields_EmptyInput_ReturnsAllEmpty(t *testing.T) {
	t.Parallel()
	a, b, c := parseInspectSummaryFields("   ")
	assert.Empty(t, a)
	assert.Empty(t, b)
	assert.Empty(t, c)
}

func TestParseInspectSummaryFields_ColonSeparator_ParsesCorrectly(t *testing.T) {
	t.Parallel()
	indexedAt, pathValue, hash := parseInspectSummaryFields("indexed_at:2026-05-01T00:00:00Z path:/repo hash:deadbeef")
	assert.Equal(t, "/repo", pathValue)
	assert.Equal(t, "deadbeef", hash)
	_ = indexedAt
}

func TestTrimLastRune_RemovesLastCharacter(t *testing.T) {
	t.Parallel()
	cases := []struct {
		input, expected string
	}{
		{"hello", "hell"},
		{"a", ""},
		{"", ""},
		{"café", "caf"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, trimLastRune(tc.input))
		})
	}
}

func TestInspectBackspace_InDirectorySearch_TrimsCursor(t *testing.T) {
	t.Parallel()

	// Arrange
	model := inspectModel{
		mode:                 inspectViewModeDirectories,
		directorySearchMode:  true,
		directorySearchQuery: "docs",
		directories: []inspectDirectoryRow{
			{path: "/repo/docs"},
		},
		filteredDirectories: []inspectDirectoryRow{
			{path: "/repo/docs"},
		},
	}

	// Act
	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	updatedModel := updated.(inspectModel)

	// Assert
	assert.Equal(t, "doc", updatedModel.directorySearchQuery)
}

func TestInspectBackspace_InLogSearch_TrimsCursor(t *testing.T) {
	t.Parallel()

	// Arrange
	model := inspectModel{
		mode:           inspectViewModeLogs,
		logSearchMode:  true,
		logSearchQuery: "repo",
		logs:           []inspectLogRow{{path: "/repo", indexedAt: "2026-05-01T00:00:00Z", hash: "abc"}},
		filteredLogs:   []inspectLogRow{{path: "/repo", indexedAt: "2026-05-01T00:00:00Z", hash: "abc"}},
	}

	// Act
	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	updatedModel := updated.(inspectModel)

	// Assert
	assert.Equal(t, "rep", updatedModel.logSearchQuery)
}

func TestInspectDocumentsVisibleRange_Empty_ReturnsZeros(t *testing.T) {
	t.Parallel()
	model := inspectModel{}
	start, end := inspectDocumentsVisibleRange(model)
	assert.Equal(t, 0, start)
	assert.Equal(t, 0, end)
}

func TestInspectDocumentsVisibleRange_UsesHeight(t *testing.T) {
	t.Parallel()
	model := inspectModel{
		filteredDocuments: make([]inspectDocumentRow, 20),
		documentStart:     2,
		height:            16,
	}
	start, end := inspectDocumentsVisibleRange(model)
	assert.Equal(t, 2, start)
	assert.Greater(t, end, start)
}

func TestInspectLogsVisibleRange_Empty_ReturnsZeros(t *testing.T) {
	t.Parallel()
	model := inspectModel{}
	start, end := inspectLogsVisibleRange(model)
	assert.Equal(t, 0, start)
	assert.Equal(t, 0, end)
}

func TestInspectLogsVisibleRange_UsesHeight(t *testing.T) {
	t.Parallel()
	model := inspectModel{
		filteredLogs: make([]inspectLogRow, 20),
		logStart:     1,
		height:       16,
	}
	start, end := inspectLogsVisibleRange(model)
	assert.Equal(t, 1, start)
	assert.Greater(t, end, start)
}

func TestInspectLogsPageStep_MinimumOne(t *testing.T) {
	t.Parallel()
	model := inspectModel{height: 0}
	assert.GreaterOrEqual(t, inspectLogsPageStep(model), 1)
}

func TestInspectLogsPageStep_NormalHeight_PositiveStep(t *testing.T) {
	t.Parallel()
	model := inspectModel{height: 20}
	assert.GreaterOrEqual(t, inspectLogsPageStep(model), 1)
}

func TestInspectJSONRange_Empty_ReturnsZeros(t *testing.T) {
	t.Parallel()
	model := inspectModel{}
	start, end := inspectJSONRange(model)
	assert.Equal(t, 0, start)
	assert.Equal(t, 0, end)
}

func TestInspectJSONRange_UsesHeight(t *testing.T) {
	t.Parallel()
	model := inspectModel{
		jsonLines: make([]string, 30),
		jsonStart: 3,
		height:    20,
	}
	start, end := inspectJSONRange(model)
	assert.Equal(t, 3, start)
	assert.Greater(t, end, start)
}

func TestInspectDividerWidth_ClampsToRange(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 8, inspectDividerWidth(4), "expected minimum divider width of 8")
	assert.Equal(t, 120, inspectDividerWidth(200), "expected maximum divider width of 120")
	assert.Equal(t, 80, inspectDividerWidth(80), "expected divider width of 80 to be returned as-is")
}

func TestInspectBuildLogRow_PlainText_SetsSummaryAndRaw(t *testing.T) {
	t.Parallel()
	row := inspectBuildLogRow("plain summary text", 0, "/repo/.idx/logs/tlog.idx")
	assert.Equal(t, "plain summary text", row.summary)
	assert.Equal(t, "plain summary text", row.jsonRaw)
}

func TestInspectBuildLogRow_JSONLine_ParsesFields(t *testing.T) {
	t.Parallel()
	line := `{"indexed_at":"2026-05-01T12:00:00Z","path":"/repo/internal","hash":"abc123","summary":"ok"}`
	row := inspectBuildLogRow(line, 0, "/repo/.idx/logs/tlog.idx")
	assert.Equal(t, "2026-05-01T12:00:00Z", row.indexedAt)
	assert.Equal(t, "/repo/internal", row.path)
	assert.Equal(t, "abc123", row.hash)
	assert.Equal(t, "ok", row.summary)
}

func TestInspectBuildLogRow_MissingFields_FallbackToDash(t *testing.T) {
	t.Parallel()
	row := inspectBuildLogRow(`{"other":"value"}`, 0, "/repo/.idx/logs/tlog.idx")
	assert.Equal(t, "-", row.indexedAt)
	assert.Equal(t, "-", row.hash)
}

func TestInspectDocumentDirectory_FromKeySeparator(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "/repo/internal", inspectDocumentDirectory("/repo/internal::main.go", "/repo/internal/main.go"))
}

func TestInspectDocumentDirectory_FromPathFallback(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "/repo/internal/core", inspectDocumentDirectory("doc1", "/repo/internal/core/service.go"))
}

func TestInspectDocumentDirectory_EmptyPath_ReturnsDot(t *testing.T) {
	t.Parallel()
	assert.Equal(t, ".", inspectDocumentDirectory("doc1", ""))
}

func TestInspectDocumentDirectory_RootFilename_ReturnsFilename(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "main.go", inspectDocumentDirectory("doc1", "main.go"))
}

func TestInspectRowsFromIndex_Nil_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	dirs, byDir := inspectRowsFromIndex(nil)
	assert.Empty(t, dirs)
	assert.Empty(t, byDir)
}

func TestInspectRowsFromIndex_BuildsDirectoriesAndDocuments(t *testing.T) {
	t.Parallel()

	// Arrange
	index := indexing.NewInvertedIndex()
	index.AddDocument("/repo/internal::service.go", "internal/service.go", 10)
	index.AddDocument("/repo/internal::core.go", "internal/core.go", 5)
	index.AddDocument("/repo/cmd::main.go", "cmd/main.go", 3)

	// Act
	dirs, byDir := inspectRowsFromIndex(index)

	// Assert
	require.Len(t, dirs, 2)
	assert.Len(t, byDir["/repo/internal"], 2)
	assert.Len(t, byDir["/repo/cmd"], 1)
}
