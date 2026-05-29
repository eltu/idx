package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"idx/internal/features/indexing"
)

func TestAdjustInspectDirectoriesViewport_KeepsSelectionVisible(t *testing.T) {
	t.Parallel()

	// Arrange
	model := inspectModel{
		directories:         make([]inspectDirectoryRow, 20),
		filteredDirectories: make([]inspectDirectoryRow, 20),
		directorySelected:   12,
		directoryStart:      0,
		height:              15,
	}

	// Act
	model = adjustInspectDirectoriesViewport(model)
	start, end := inspectDirectoriesVisibleRange(model)

	// Assert
	assert.GreaterOrEqual(t, model.directorySelected, start)
	assert.Less(t, model.directorySelected, end)
}

func TestInspectDirectoriesVisibleRange_UsesViewportWindow(t *testing.T) {
	t.Parallel()

	// Arrange
	model := inspectModel{
		directories:         make([]inspectDirectoryRow, 10),
		filteredDirectories: make([]inspectDirectoryRow, 10),
		directorySelected:   6,
		directoryStart:      4,
		height:              13,
	}

	// Act
	start, end := inspectDirectoriesVisibleRange(model)

	// Assert
	assert.Equal(t, 4, start)
	assert.Equal(t, 6, end)
}

func TestInspectTruncateLine_RespectsWidth(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "123...", inspectTruncateLine("123456789", 6))
}

func TestInspectTruncateLine_NoTruncationWhenFits(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "short", inspectTruncateLine("short", 10))
}

func TestInspectEnter_OpensDirectoryThenJSONAndEscReturnsToDocuments(t *testing.T) {
	t.Parallel()

	// Arrange
	index := indexing.NewInvertedIndex()
	index.AddDocument("/repo/internal::doc1", "internal/doc1.go", 8)
	index.AddTerm("alpha", "/repo/internal::doc1", 2, []int{1, 4})
	model := newInspectModel(index)
	assert.Equal(t, inspectViewModeDirectories, model.mode)

	// Act — open directory
	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	documentsModel := updated.(inspectModel)
	assert.Equal(t, inspectViewModeDocuments, documentsModel.mode)

	// Act — open document (JSON)
	updated, _ = documentsModel.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	jsonModel := updated.(inspectModel)
	assert.Equal(t, inspectViewModeJSON, jsonModel.mode)
	assert.NotEmpty(t, jsonModel.jsonLines)

	// Act — Escape returns to documents
	updated, _ = jsonModel.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	documentsModel = updated.(inspectModel)
	assert.Equal(t, inspectViewModeDocuments, documentsModel.mode)
}

func TestInspectDocumentJSON_IncludesDocumentFieldsAndTerms(t *testing.T) {
	t.Parallel()

	// Arrange
	index := indexing.NewInvertedIndex()
	index.AddDocument("doc1", "internal/doc1.go", 8)
	index.AddTerm("beta", "doc1", 1, []int{3})
	index.AddTerm("alpha", "doc1", 2, []int{1, 4})

	// Act
	jsonText, err := inspectDocumentJSON(index, inspectDocumentRow{key: "doc1"})

	// Assert
	require.NoError(t, err)
	for _, snippet := range []string{`"name": "doc1"`, `"path": "internal/doc1.go"`, `"uniqueTerms": 2`, `"term": "alpha"`, `"term": "beta"`} {
		assert.Contains(t, jsonText, snippet)
	}
}

func TestInspectJSONStringIsKey_DetectsKeyPosition(t *testing.T) {
	t.Parallel()

	line := `  "name": "value"`
	_, end := inspectReadJSONString(line, 2)
	assert.True(t, inspectJSONStringIsKey(line, end))
}

func TestInspectReadJSONKeyword_ParsesTrueKeyword(t *testing.T) {
	t.Parallel()

	token, _, ok := inspectReadJSONKeyword("  true,", 2)
	assert.True(t, ok)
	assert.Equal(t, "true", token)
}

func TestInspectReadJSONKeyword_NoMatchForWordPrefix(t *testing.T) {
	t.Parallel()

	_, _, ok := inspectReadJSONKeyword("truename", 0)
	assert.False(t, ok)
}

func TestInspectReadJSONNumber_DetectsNumericStart(t *testing.T) {
	t.Parallel()

	assert.True(t, inspectStartsJSONNumber("-12.5e+3", 0))

	token, end := inspectReadJSONNumber("-12.5e+3,", 0)
	assert.Equal(t, "-12.5e+3", token)
	assert.Equal(t, 8, end)
}

func TestInspectList_PgDown_MovesByPage(t *testing.T) {
	t.Parallel()

	// Arrange
	model := inspectModel{
		directories:         make([]inspectDirectoryRow, 20),
		filteredDirectories: make([]inspectDirectoryRow, 20),
		directorySelected:   0,
		directoryStart:      0,
		height:              16,
		mode:                inspectViewModeDirectories,
	}

	// Act
	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	result := updated.(inspectModel)

	// Assert
	assert.Equal(t, 4, result.directorySelected)
}

func TestInspectList_PgUp_MovesByPage(t *testing.T) {
	t.Parallel()

	// Arrange
	model := inspectModel{
		documents:         make([]inspectDocumentRow, 20),
		filteredDocuments: make([]inspectDocumentRow, 20),
		documentSelected:  9,
		documentStart:     5,
		height:            16,
		mode:              inspectViewModeDocuments,
	}

	// Act
	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyPgUp})
	result := updated.(inspectModel)

	// Assert
	assert.Equal(t, 6, result.documentSelected)
}

func TestInspectDirectory_SlashSearch_FiltersRows(t *testing.T) {
	t.Parallel()

	// Arrange
	model := inspectModel{
		directories: []inspectDirectoryRow{
			{path: "/repo/internal", documentCount: 2},
			{path: "/repo/docs", documentCount: 1},
		},
		filteredDirectories: []inspectDirectoryRow{
			{path: "/repo/internal", documentCount: 2},
			{path: "/repo/docs", documentCount: 1},
		},
		mode: inspectViewModeDirectories,
	}

	// Act — enter search mode
	updated, _ := model.Update(tea.KeyPressMsg{Text: "/"})
	updatedModel := updated.(inspectModel)
	assert.True(t, updatedModel.directorySearchMode)

	// Act — type filter
	updated, _ = updatedModel.Update(tea.KeyPressMsg{Text: "doc"})
	updatedModel = updated.(inspectModel)

	// Assert
	require.Len(t, updatedModel.filteredDirectories, 1)
	assert.Equal(t, "/repo/docs", updatedModel.filteredDirectories[0].path)
}

func TestInspectDocument_SlashSearchEscIgnoredEnterFinishes(t *testing.T) {
	t.Parallel()

	// Arrange
	model := inspectModel{
		documents: []inspectDocumentRow{
			{name: "main", path: "cmd/idx/main.go"},
			{name: "search", path: "internal/search/service.go"},
		},
		filteredDocuments: []inspectDocumentRow{
			{name: "main", path: "cmd/idx/main.go"},
			{name: "search", path: "internal/search/service.go"},
		},
		mode: inspectViewModeDocuments,
	}

	// Act — enter search and type
	updated, _ := model.Update(tea.KeyPressMsg{Text: "/"})
	updatedModel := updated.(inspectModel)
	updated, _ = updatedModel.Update(tea.KeyPressMsg{Text: "search"})
	updatedModel = updated.(inspectModel)
	require.Len(t, updatedModel.filteredDocuments, 1)

	// Act — Esc should keep filter active
	updated, _ = updatedModel.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	updatedModel = updated.(inspectModel)
	assert.True(t, updatedModel.documentSearchMode)
	require.Len(t, updatedModel.filteredDocuments, 1)

	// Act — Enter exits search mode
	updated, _ = updatedModel.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updatedModel = updated.(inspectModel)
	assert.False(t, updatedModel.documentSearchMode)
}

func TestInspectDocuments_Esc_ReturnsToDirectories(t *testing.T) {
	t.Parallel()

	// Arrange
	model := inspectModel{
		mode:              inspectViewModeDocuments,
		documents:         []inspectDocumentRow{{name: "main", path: "cmd/idx/main.go"}},
		filteredDocuments: []inspectDocumentRow{{name: "main", path: "cmd/idx/main.go"}},
	}

	// Act
	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	result := updated.(inspectModel)

	// Assert
	assert.Equal(t, inspectViewModeDirectories, result.mode)
}

func TestInspectLogs_Esc_DoesNotLeaveMode(t *testing.T) {
	t.Parallel()

	// Arrange
	log := inspectLogRow{indexedAt: "2026-04-30T10:00:00Z", path: "/repo", hash: "abc123", summary: "ok"}
	model := inspectModel{
		mode:         inspectViewModeLogs,
		logs:         []inspectLogRow{log},
		filteredLogs: []inspectLogRow{log},
		logSelected:  0,
	}

	// Act
	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	result := updated.(inspectModel)

	// Assert
	assert.Equal(t, inspectViewModeLogs, result.mode)
}

func TestInspectCommandMode_TabAutocomplete_SingleMatch(t *testing.T) {
	t.Parallel()

	// Arrange
	model := inspectModel{mode: inspectViewModeDirectories}

	// Act
	updated, _ := model.Update(tea.KeyPressMsg{Text: ":"})
	updatedModel := updated.(inspectModel)
	updated, _ = updatedModel.Update(tea.KeyPressMsg{Text: "tl"})
	updatedModel = updated.(inspectModel)
	updated, _ = updatedModel.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	updatedModel = updated.(inspectModel)

	// Assert
	assert.Equal(t, "tlog", updatedModel.commandQuery)
}

func TestInspectJSONMode_AllowsCommandInput(t *testing.T) {
	t.Parallel()

	// Arrange
	model := inspectModel{mode: inspectViewModeJSON, jsonLines: []string{"{}"}}

	// Act
	updated, _ := model.Update(tea.KeyPressMsg{Text: ":"})
	result := updated.(inspectModel)

	// Assert
	assert.Equal(t, inspectCommandModeCommand, result.commandMode)
}

func TestInspectCommandMode_SwitchesToLogsNavigator(t *testing.T) {
	t.Parallel()

	// Arrange
	model := newInspectModel(indexing.NewInvertedIndex())

	// Act — enter command mode
	updated, _ := model.Update(tea.KeyPressMsg{Text: ":"})
	updatedModel := updated.(inspectModel)
	assert.Equal(t, inspectCommandModeCommand, updatedModel.commandMode)

	// Act — type "tlog" and enter
	updated, _ = updatedModel.Update(tea.KeyPressMsg{Text: "tlog"})
	updatedModel = updated.(inspectModel)
	updated, _ = updatedModel.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updatedModel = updated.(inspectModel)

	// Assert
	assert.Equal(t, inspectViewModeLogs, updatedModel.mode)
}

func TestInspectLogs_Enter_DoesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	log := inspectLogRow{indexedAt: "2026-04-30T10:00:00Z", path: "/repo", hash: "abc123", summary: "ok"}
	model := inspectModel{
		mode:         inspectViewModeLogs,
		logs:         []inspectLogRow{log},
		filteredLogs: []inspectLogRow{log},
		logSelected:  0,
	}

	// Act
	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	result := updated.(inspectModel)

	// Assert
	assert.Equal(t, inspectViewModeLogs, result.mode)
}

func TestInspectCommandMode_SwitchesToIndexNavigator(t *testing.T) {
	t.Parallel()

	// Arrange
	model := inspectModel{mode: inspectViewModeLogs}

	// Act
	updated, _ := model.Update(tea.KeyPressMsg{Text: ":"})
	updatedModel := updated.(inspectModel)
	updated, _ = updatedModel.Update(tea.KeyPressMsg{Text: "index"})
	updatedModel = updated.(inspectModel)
	updated, _ = updatedModel.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updatedModel = updated.(inspectModel)

	// Assert
	assert.Equal(t, inspectViewModeDirectories, updatedModel.mode)
}

func TestInspectCommandMode_UnknownCommand_SetsError(t *testing.T) {
	t.Parallel()

	// Arrange
	model := newInspectModel(indexing.NewInvertedIndex())

	// Act
	updated, _ := model.Update(tea.KeyPressMsg{Text: ":"})
	updatedModel := updated.(inspectModel)
	updated, _ = updatedModel.Update(tea.KeyPressMsg{Text: "invalid"})
	updatedModel = updated.(inspectModel)
	updated, _ = updatedModel.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	updatedModel = updated.(inspectModel)

	// Assert
	assert.NotEmpty(t, updatedModel.commandError)
}

func TestInspectLogTable_UsesFixedSeparators(t *testing.T) {
	t.Parallel()

	// Assert header
	header := inspectLogTableHeader()
	assert.Contains(t, header, " | ")

	// Assert row
	row := inspectLogTableRow(inspectLogRow{indexedAt: "2026-04-30T10:00:00Z", path: "/repo", hash: "abc123"})
	assert.Contains(t, row, " | ")
}

func TestInspectLogs_HorizontalNavigation_WithArrows(t *testing.T) {
	t.Parallel()

	// Arrange
	log := inspectLogRow{indexedAt: "2026-04-30T10:00:00Z", path: "/a/very/long/path", hash: "1234567890abcdef", summary: "summary"}
	model := inspectModel{
		mode:         inspectViewModeLogs,
		width:        20,
		logs:         []inspectLogRow{log},
		filteredLogs: []inspectLogRow{log},
	}

	// Act — right
	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	updatedModel := updated.(inspectModel)
	assert.Positive(t, updatedModel.logColumnOffset)

	// Act — left
	updated, _ = updatedModel.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	updatedModel = updated.(inspectModel)
	assert.Equal(t, 0, updatedModel.logColumnOffset)
}

func TestReplaceInspectLogs_PreservesSelection(t *testing.T) {
	t.Parallel()

	// Arrange
	model := inspectModel{
		mode:         inspectViewModeLogs,
		logs:         []inspectLogRow{{indexedAt: "a", jsonRaw: `{"id":1}`}, {indexedAt: "b", jsonRaw: `{"id":2}`}},
		filteredLogs: []inspectLogRow{{indexedAt: "a", jsonRaw: `{"id":1}`}, {indexedAt: "b", jsonRaw: `{"id":2}`}},
		logSelected:  1,
	}

	// Act
	updated := replaceInspectLogs(model, []inspectLogRow{
		{indexedAt: "b", jsonRaw: `{"id":2}`},
		{indexedAt: "c", jsonRaw: `{"id":3}`},
	})

	// Assert
	require.Len(t, updated.filteredLogs, 2)
	assert.Equal(t, 0, updated.logSelected)
}

func TestParseInspectSummaryFields_ParsesAllFields(t *testing.T) {
	t.Parallel()

	// Act
	indexedAt, pathValue, hash := parseInspectSummaryFields("indexed_at=2026-04-30T10:00:00Z path=/repo/internal hash=abcdef")

	// Assert
	assert.Equal(t, "2026-04-30T10:00:00Z", indexedAt)
	assert.Equal(t, "/repo/internal", pathValue)
	assert.Equal(t, "abcdef", hash)
}

func TestSortInspectLogsNewestFirst_SortsDescending(t *testing.T) {
	t.Parallel()

	// Arrange
	rows := []inspectLogRow{
		{indexedAt: "2026-04-29T10:00:00Z", path: "/repo/a"},
		{indexedAt: "2026-04-30T10:00:00Z", path: "/repo/b"},
		{indexedAt: "2026-04-28T10:00:00Z", path: "/repo/c"},
	}

	// Act
	sortInspectLogsNewestFirst(rows)

	// Assert
	assert.Equal(t, "2026-04-30T10:00:00Z", rows[0].indexedAt)
	assert.Equal(t, "2026-04-29T10:00:00Z", rows[1].indexedAt)
	assert.Equal(t, "2026-04-28T10:00:00Z", rows[2].indexedAt)
}

func TestSortInspectLogsNewestFirst_ParseableBeforeUnknown(t *testing.T) {
	t.Parallel()

	// Arrange
	rows := []inspectLogRow{
		{indexedAt: "-", path: "/repo/a"},
		{indexedAt: "2026-04-30T10:00:00Z", path: "/repo/b"},
		{indexedAt: "unknown", path: "/repo/c"},
	}

	// Act
	sortInspectLogsNewestFirst(rows)

	// Assert
	assert.Equal(t, "2026-04-30T10:00:00Z", rows[0].indexedAt)
}
