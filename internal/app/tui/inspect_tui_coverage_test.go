package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"idx/internal/features/indexing"
)

// --- clampInt ---

func TestClampInt_BelowLow_ReturnsLow(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 0, clampInt(-5, 0, 10))
}

func TestClampInt_AboveHigh_ReturnsHigh(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 10, clampInt(15, 0, 10))
}

func TestClampInt_WithinRange_ReturnsValue(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 5, clampInt(5, 0, 10))
}

// --- inspectInputLine ---

func TestInspectInputLine_ShowsCommandError(t *testing.T) {
	t.Parallel()

	// Arrange
	model := inspectModel{commandMode: inspectCommandModeNone, commandError: "unknown command \"bad\""}

	// Act
	line := inspectInputLine(model, false, "")

	// Assert
	assert.Contains(t, line, "unknown command")
}

func TestInspectInputLine_ShowsNonEmptySearchQuery(t *testing.T) {
	t.Parallel()

	// Arrange
	model := inspectModel{commandMode: inspectCommandModeNone}

	// Act
	line := inspectInputLine(model, false, "myquery")

	// Assert
	assert.Contains(t, line, "myquery")
}

func TestInspectInputLine_CommandModeWithSuggestions(t *testing.T) {
	t.Parallel()

	// Arrange
	model := inspectModel{commandMode: inspectCommandModeCommand, commandQuery: "ind"}

	// Act
	line := inspectInputLine(model, false, "")

	// Assert
	assert.Contains(t, line, "ind")
}

func TestInspectInputLine_CommandModeNoSuggestions(t *testing.T) {
	t.Parallel()

	// Arrange
	model := inspectModel{commandMode: inspectCommandModeCommand, commandQuery: "zzz"}

	// Act
	line := inspectInputLine(model, false, "")

	// Assert
	assert.Contains(t, line, "zzz")
}

// --- inspectHorizontalWindow ---

func TestInspectHorizontalWindow_ZeroWidth_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	assert.Empty(t, inspectHorizontalWindow("hello", 0, 0))
}

func TestInspectHorizontalWindow_OffsetBeyondText_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	assert.Empty(t, inspectHorizontalWindow("hello", 5, 100))
}

func TestInspectHorizontalWindow_NegativeOffset_TreatedAsZero(t *testing.T) {
	t.Parallel()

	assert.True(t, strings.HasPrefix(inspectHorizontalWindow("hello", 3, -1), "hel"))
}

// --- adjustInspectJSONViewport ---

func TestAdjustInspectJSONViewport_ClampsJsonStart(t *testing.T) {
	t.Parallel()

	// Arrange
	model := inspectModel{jsonLines: []string{"a", "b", "c", "d", "e"}, jsonStart: 100, height: 10}

	// Act
	model = adjustInspectJSONViewport(model)

	// Assert
	assert.GreaterOrEqual(t, model.jsonStart, 0)
	assert.LessOrEqual(t, model.jsonStart, len(model.jsonLines))
}

func TestAdjustInspectJSONViewport_EmptyLines_ResetsToZero(t *testing.T) {
	t.Parallel()

	model := inspectModel{jsonLines: nil, jsonStart: 5}
	model = adjustInspectJSONViewport(model)
	assert.Equal(t, 0, model.jsonStart)
}

// --- inspectJSONRange ---

func TestInspectJSONRange_StartBeyondEnd_ClampsToLastLine(t *testing.T) {
	t.Parallel()

	model := inspectModel{jsonLines: []string{"line1", "line2", "line3"}, jsonStart: 999, height: 1}
	start, end := inspectJSONRange(model)
	assert.Less(t, start, end)
}

// --- inspectReadJSONString ---

func TestInspectReadJSONString_EscapedQuote(t *testing.T) {
	t.Parallel()

	line := `"hello \"world\""`
	token, _ := inspectReadJSONString(line, 0)
	assert.True(t, strings.HasPrefix(token, `"`))
}

func TestInspectReadJSONString_Unterminated(t *testing.T) {
	t.Parallel()

	line := `"unterminated`
	token, end := inspectReadJSONString(line, 0)
	assert.NotEmpty(t, token)
	assert.Equal(t, len(line), end)
}

// --- adjustInspectDocumentsViewport ---

func TestAdjustInspectDocumentsViewport_EmptyResetsSelection(t *testing.T) {
	t.Parallel()

	model := inspectModel{filteredDocuments: nil, documentStart: 3, documentSelected: 5}
	model = adjustInspectDocumentsViewport(model)
	assert.Equal(t, 0, model.documentStart)
	assert.Equal(t, 0, model.documentSelected)
}

func TestAdjustInspectDocumentsViewport_ScrollsDown(t *testing.T) {
	t.Parallel()

	docs := make([]inspectDocumentRow, 20)
	model := inspectModel{filteredDocuments: docs, documentSelected: 15, documentStart: 0, height: 10}
	model = adjustInspectDocumentsViewport(model)
	assert.LessOrEqual(t, model.documentStart, model.documentSelected)
}

func TestAdjustInspectDocumentsViewport_ScrollsUp(t *testing.T) {
	t.Parallel()

	docs := make([]inspectDocumentRow, 20)
	model := inspectModel{filteredDocuments: docs, documentSelected: 2, documentStart: 10, height: 20}
	model = adjustInspectDocumentsViewport(model)
	assert.LessOrEqual(t, model.documentStart, model.documentSelected)
}

// --- updateInspectDocumentsSelection ---

func TestUpdateInspectDocumentsSelection_PgUp_Decrements(t *testing.T) {
	t.Parallel()

	docs := make([]inspectDocumentRow, 20)
	model := inspectModel{filteredDocuments: docs, documentSelected: 10, height: 10}
	model = updateInspectDocumentsSelection(model, "pgup")
	assert.Less(t, model.documentSelected, 10)
}

func TestUpdateInspectDocumentsSelection_PgUp_ClampsToZero(t *testing.T) {
	t.Parallel()

	docs := make([]inspectDocumentRow, 5)
	model := inspectModel{filteredDocuments: docs, documentSelected: 1, height: 20}
	model = updateInspectDocumentsSelection(model, "pgup")
	assert.GreaterOrEqual(t, model.documentSelected, 0)
}

func TestUpdateInspectDocumentsSelection_PgDown_Increments(t *testing.T) {
	t.Parallel()

	docs := make([]inspectDocumentRow, 20)
	model := inspectModel{filteredDocuments: docs, documentSelected: 0, height: 10}
	model = updateInspectDocumentsSelection(model, "pgdown")
	assert.Positive(t, model.documentSelected)
}

func TestUpdateInspectDocumentsSelection_PgDown_ClampsToMax(t *testing.T) {
	t.Parallel()

	docs := make([]inspectDocumentRow, 5)
	model := inspectModel{filteredDocuments: docs, documentSelected: 4, height: 20}
	model = updateInspectDocumentsSelection(model, "pgdown")
	assert.Less(t, model.documentSelected, len(docs))
}

// --- updateInspectDirectorySearchMode ---

func newDirectorySearchModel() inspectModel {
	return inspectModel{
		mode:                inspectViewModeDirectories,
		directorySearchMode: true,
		directories:         []inspectDirectoryRow{{path: "/repo/internal"}, {path: "/repo/cmd"}},
		filteredDirectories: []inspectDirectoryRow{{path: "/repo/internal"}, {path: "/repo/cmd"}},
	}
}

func TestUpdateInspectDirectorySearchMode_KeyRunes_AppendsQuery(t *testing.T) {
	t.Parallel()

	model := newDirectorySearchModel()
	result, _ := updateInspectDirectorySearchMode(model, tea.KeyPressMsg{Text: "int"})
	assert.Equal(t, "int", result.(inspectModel).directorySearchQuery)
}

func TestUpdateInspectDirectorySearchMode_Enter_ExitsSearch(t *testing.T) {
	t.Parallel()

	model := newDirectorySearchModel()
	model.directorySearchQuery = "int"
	result, _ := updateInspectDirectorySearchMode(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.False(t, result.(inspectModel).directorySearchMode)
}

func TestUpdateInspectDirectorySearchMode_CtrlC_Quits(t *testing.T) {
	t.Parallel()

	model := newDirectorySearchModel()
	result, _ := updateInspectDirectorySearchMode(model, tea.KeyPressMsg{Text: "ctrl+c"})
	assert.True(t, result.(inspectModel).quitting)
}

// --- updateInspectLogSearchMode ---

func newLogSearchModel() inspectModel {
	return inspectModel{
		mode:          inspectViewModeLogs,
		logSearchMode: true,
		logs:          []inspectLogRow{{path: "/repo", indexedAt: "2026-05-01", hash: "abc"}},
		filteredLogs:  []inspectLogRow{{path: "/repo", indexedAt: "2026-05-01", hash: "abc"}},
	}
}

func TestUpdateInspectLogSearchMode_Enter_ExitsSearch(t *testing.T) {
	t.Parallel()

	model := newLogSearchModel()
	model.logSearchQuery = "repo"
	result, _ := updateInspectLogSearchMode(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.False(t, result.(inspectModel).logSearchMode)
}

func TestUpdateInspectLogSearchMode_KeyRunes_AppendsQuery(t *testing.T) {
	t.Parallel()

	model := newLogSearchModel()
	result, _ := updateInspectLogSearchMode(model, tea.KeyPressMsg{Text: "rep"})
	assert.Equal(t, "rep", result.(inspectModel).logSearchQuery)
}

func TestUpdateInspectLogSearchMode_Backspace_Trims(t *testing.T) {
	t.Parallel()

	model := newLogSearchModel()
	model.logSearchQuery = "repo"
	result, _ := updateInspectLogSearchMode(model, tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Equal(t, "rep", result.(inspectModel).logSearchQuery)
}

func TestUpdateInspectLogSearchMode_CtrlC_Quits(t *testing.T) {
	t.Parallel()

	model := newLogSearchModel()
	result, _ := updateInspectLogSearchMode(model, tea.KeyPressMsg{Text: "ctrl+c"})
	assert.True(t, result.(inspectModel).quitting)
}

// --- autocompleteInspectCommand multi-match path ---

func TestAutocompleteInspectCommand_MultiMatch_ExpandsCommonPrefix(t *testing.T) {
	t.Parallel()

	// Temporarily add a second command with a common prefix to trigger multi-match path.
	original := inspectAvailableCommands
	inspectAvailableCommands = []string{"inspect", "index"}
	defer func() { inspectAvailableCommands = original }()

	// "in" matches both "inspect" and "index", common prefix is "in"
	// len("in") == len("in"), so returns query
	result := autocompleteInspectCommand(":in")
	assert.Equal(t, ":in", result)
}

func TestAutocompleteInspectCommand_MultiMatch_ExtendsPrefix(t *testing.T) {
	t.Parallel()

	original := inspectAvailableCommands
	inspectAvailableCommands = []string{"inspect", "inspector"}
	defer func() { inspectAvailableCommands = original }()

	// "ins" matches both, common prefix is "inspect" which is longer than "ins"
	result := autocompleteInspectCommand(":ins")
	assert.Equal(t, "inspect", result)
}

func TestAutocompleteInspectCommand_MultiMatch_NoCommonPrefix(t *testing.T) {
	t.Parallel()

	original := inspectAvailableCommands
	inspectAvailableCommands = []string{"alpha", "beta"}
	defer func() { inspectAvailableCommands = original }()

	// "a" only matches "alpha" → single match
	result := autocompleteInspectCommand(":a")
	assert.Equal(t, "alpha", result)
}

// --- inspectDocumentJSON ---

func TestInspectDocumentJSON_NilIndex_ReturnsError(t *testing.T) {
	t.Parallel()

	_, err := inspectDocumentJSON(nil, inspectDocumentRow{key: "dir::file.go"})
	require.Error(t, err)
}

func TestInspectDocumentJSON_MissingDocument_ReturnsError(t *testing.T) {
	t.Parallel()

	index := indexing.NewInvertedIndex()
	_, err := inspectDocumentJSON(index, inspectDocumentRow{key: "dir::missing.go"})
	require.Error(t, err)
}

func TestInspectDocumentJSON_ValidDocument_ReturnsJSON(t *testing.T) {
	t.Parallel()

	// Arrange
	index := indexing.NewInvertedIndex()
	index.AddDocument("dir::file.go", "dir/file.go", 42)

	// Act
	result, err := inspectDocumentJSON(index, inspectDocumentRow{key: "dir::file.go", name: "file.go", path: "dir/file.go"})

	// Assert
	require.NoError(t, err)
	assert.Contains(t, result, "file.go")
}

// --- inspectJSONStringIsKey ---

func TestInspectJSONStringIsKey_ReturnsFalseWhenNoColon(t *testing.T) {
	t.Parallel()

	line := `"value" , "next"`
	assert.False(t, inspectJSONStringIsKey(line, 7))
}

func TestInspectJSONStringIsKey_ReturnsFalseAtEnd(t *testing.T) {
	t.Parallel()

	line := `"value"`
	assert.False(t, inspectJSONStringIsKey(line, len(line)))
}

// --- replaceInspectLogs selection preservation ---

func TestReplaceInspectLogs_PreservesSelectedRow(t *testing.T) {
	t.Parallel()

	// Arrange
	logs := []inspectLogRow{{path: "/a", jsonRaw: "raw-a"}, {path: "/b", jsonRaw: "raw-b"}}
	model := inspectModel{logs: logs, filteredLogs: logs, logSelected: 1}
	newLogs := []inspectLogRow{{path: "/c", jsonRaw: "raw-c"}, {path: "/b", jsonRaw: "raw-b"}}

	// Act
	model = replaceInspectLogs(model, newLogs)

	// Assert
	require.True(t, model.logSelected >= 0 && model.logSelected < len(model.filteredLogs))
	assert.Equal(t, "raw-b", model.filteredLogs[model.logSelected].jsonRaw)
}

// --- updateInspectJSONMode ---

func newJSONModel() inspectModel {
	// 30 lines ensures scrolling works with height=20 (inspectJSONHeight = max(20-7,1)=13)
	lines := make([]string, 30)
	for i := range lines {
		lines[i] = "line"
	}
	return inspectModel{mode: inspectViewModeJSON, jsonLines: lines, jsonStart: 2, height: 20, width: 80}
}

func TestUpdateInspectJSONMode_CtrlC_Quits(t *testing.T) {
	t.Parallel()

	result, _ := updateInspectJSONMode(newJSONModel(), tea.KeyPressMsg{Text: "ctrl+c"})
	assert.True(t, result.(inspectModel).quitting)
}

func TestUpdateInspectJSONMode_Q_Quits(t *testing.T) {
	t.Parallel()

	result, _ := updateInspectJSONMode(newJSONModel(), tea.KeyPressMsg{Text: "q"})
	assert.True(t, result.(inspectModel).quitting)
}

func TestUpdateInspectJSONMode_Esc_ReturnsToDocuments(t *testing.T) {
	t.Parallel()

	model := newJSONModel()
	model.jsonReturnMode = inspectViewModeDocuments
	result, _ := updateInspectJSONMode(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.Equal(t, inspectViewModeDocuments, result.(inspectModel).mode)
}

func TestUpdateInspectJSONMode_Esc_ReturnsToLogs(t *testing.T) {
	t.Parallel()

	model := newJSONModel()
	model.jsonReturnMode = inspectViewModeLogs
	result, _ := updateInspectJSONMode(model, tea.KeyPressMsg{Code: tea.KeyEscape})
	assert.Equal(t, inspectViewModeLogs, result.(inspectModel).mode)
}

func TestUpdateInspectJSONMode_K_DecrementsStart(t *testing.T) {
	t.Parallel()

	model := newJSONModel()
	model.jsonStart = 3
	result, _ := updateInspectJSONMode(model, tea.KeyPressMsg{Text: "k"})
	assert.Less(t, result.(inspectModel).jsonStart, 3)
}

func TestUpdateInspectJSONMode_J_IncrementsStart(t *testing.T) {
	t.Parallel()

	model := newJSONModel()
	model.jsonStart = 0
	result, _ := updateInspectJSONMode(model, tea.KeyPressMsg{Text: "j"})
	assert.Positive(t, result.(inspectModel).jsonStart)
}

func TestUpdateInspectJSONMode_PgUp_DecrementsStart(t *testing.T) {
	t.Parallel()

	model := newJSONModel()
	model.jsonStart = 3
	result, _ := updateInspectJSONMode(model, tea.KeyPressMsg{Code: tea.KeyPgUp})
	assert.Less(t, result.(inspectModel).jsonStart, 3)
}

func TestUpdateInspectJSONMode_PgDown_IncrementsStart(t *testing.T) {
	t.Parallel()

	model := newJSONModel()
	model.jsonStart = 0
	result, _ := updateInspectJSONMode(model, tea.KeyPressMsg{Code: tea.KeyPgDown})
	assert.Positive(t, result.(inspectModel).jsonStart)
}

// --- updateInspectDocumentSearchMode ---

func newDocSearchModel() inspectModel {
	return inspectModel{
		mode:               inspectViewModeDocuments,
		documentSearchMode: true,
		documents: []inspectDocumentRow{
			{name: "service.go", path: "/repo/internal/service.go"},
			{name: "main.go", path: "/repo/cmd/main.go"},
		},
		filteredDocuments: []inspectDocumentRow{
			{name: "service.go", path: "/repo/internal/service.go"},
			{name: "main.go", path: "/repo/cmd/main.go"},
		},
	}
}

func TestUpdateInspectDocumentSearchMode_Enter_ExitsSearch(t *testing.T) {
	t.Parallel()

	model := newDocSearchModel()
	model.documentSearchQuery = "srv"
	result, _ := updateInspectDocumentSearchMode(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.False(t, result.(inspectModel).documentSearchMode)
}

func TestUpdateInspectDocumentSearchMode_KeyRunes_AppendsQuery(t *testing.T) {
	t.Parallel()

	model := newDocSearchModel()
	result, _ := updateInspectDocumentSearchMode(model, tea.KeyPressMsg{Text: "srv"})
	assert.Equal(t, "srv", result.(inspectModel).documentSearchQuery)
}

func TestUpdateInspectDocumentSearchMode_CtrlC_Quits(t *testing.T) {
	t.Parallel()

	model := newDocSearchModel()
	result, _ := updateInspectDocumentSearchMode(model, tea.KeyPressMsg{Text: "ctrl+c"})
	assert.True(t, result.(inspectModel).quitting)
}

// --- updateInspectDirectoriesMode navigation ---

func newDirectoriesModel() inspectModel {
	return inspectModel{
		mode: inspectViewModeDirectories,
		filteredDirectories: []inspectDirectoryRow{
			{path: "/repo/a"}, {path: "/repo/b"}, {path: "/repo/c"}, {path: "/repo/d"}, {path: "/repo/e"},
		},
		directorySelected: 2,
		height:            20,
		width:             80,
	}
}

func TestUpdateInspectDirectoriesMode_CtrlC_Quits(t *testing.T) {
	t.Parallel()

	result, _ := updateInspectDirectoriesMode(newDirectoriesModel(), tea.KeyPressMsg{Text: "ctrl+c"})
	assert.True(t, result.(inspectModel).quitting)
}

func TestUpdateInspectDirectoriesMode_PgUp_Decrements(t *testing.T) {
	t.Parallel()

	model := newDirectoriesModel()
	model.directorySelected = 4
	result, _ := updateInspectDirectoriesMode(model, tea.KeyPressMsg{Code: tea.KeyPgUp})
	assert.Less(t, result.(inspectModel).directorySelected, 4)
}

func TestUpdateInspectDirectoriesMode_PgDown_Increments(t *testing.T) {
	t.Parallel()

	model := newDirectoriesModel()
	model.directorySelected = 0
	result, _ := updateInspectDirectoriesMode(model, tea.KeyPressMsg{Code: tea.KeyPgDown})
	assert.Positive(t, result.(inspectModel).directorySelected)
}

func TestUpdateInspectDirectoriesMode_EnterOnEmpty_IsNoop(t *testing.T) {
	t.Parallel()

	model := newDirectoriesModel()
	model.filteredDirectories = nil
	result, _ := updateInspectDirectoriesMode(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Equal(t, inspectViewModeDirectories, result.(inspectModel).mode)
}

// --- inspectTruncateLine ---

func TestInspectTruncateLine_ZeroWidth_ReturnsEmpty(t *testing.T) {
	t.Parallel()

	assert.Empty(t, inspectTruncateLine("hello", 0))
}

func TestInspectTruncateLine_WidthThreeOrLess_HardTruncate(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "hel", inspectTruncateLine("hello world", 3))
}

func TestInspectTruncateLine_ExactLength_NoTruncation(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "hello", inspectTruncateLine("hello", 5))
}

func TestInspectTruncateLine_LongerThanWidth_AddsEllipsis(t *testing.T) {
	t.Parallel()

	got := inspectTruncateLine("hello world", 8)
	assert.True(t, strings.HasSuffix(got, "..."))
	assert.Equal(t, 8, len([]rune(got)))
}

// --- visible range clamps ---

func TestInspectDirectoriesVisibleRange_StartBeyondEnd_ClampsToLast(t *testing.T) {
	t.Parallel()

	model := inspectModel{
		filteredDirectories: []inspectDirectoryRow{{path: "/a"}, {path: "/b"}, {path: "/c"}},
		directoryStart:      999,
		height:              1,
	}
	start, end := inspectDirectoriesVisibleRange(model)
	assert.Less(t, start, end)
}

func TestInspectDocumentsVisibleRange_StartBeyondEnd_ClampsToLast(t *testing.T) {
	t.Parallel()

	model := inspectModel{
		filteredDocuments: []inspectDocumentRow{{name: "a"}, {name: "b"}},
		documentStart:     999,
		height:            1,
	}
	start, end := inspectDocumentsVisibleRange(model)
	assert.Less(t, start, end)
}

func TestInspectLogsVisibleRange_StartBeyondEnd_ClampsToLast(t *testing.T) {
	t.Parallel()

	model := inspectModel{
		filteredLogs: []inspectLogRow{{path: "/a"}, {path: "/b"}},
		logStart:     999,
		height:       1,
	}
	start, end := inspectLogsVisibleRange(model)
	assert.Less(t, start, end)
}

// --- Update WindowSizeMsg ---

func TestUpdate_WindowSizeMsg_UpdatesDimensions(t *testing.T) {
	t.Parallel()

	model := newInspectModel(indexing.NewInvertedIndex())
	result, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	updated := result.(inspectModel)
	assert.Equal(t, 120, updated.width)
	assert.Equal(t, 40, updated.height)
}

func TestUpdate_WindowSizeMsg_InJSONMode_UpdatesWidth(t *testing.T) {
	t.Parallel()

	model := newInspectModel(indexing.NewInvertedIndex())
	model.mode = inspectViewModeJSON
	model.jsonLines = []string{"line1", "line2"}
	result, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	assert.Equal(t, 100, result.(inspectModel).width)
}

func TestUpdate_WindowSizeMsg_InDocumentsMode_UpdatesHeight(t *testing.T) {
	t.Parallel()

	model := newInspectModel(indexing.NewInvertedIndex())
	model.mode = inspectViewModeDocuments
	result, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	assert.Equal(t, 24, result.(inspectModel).height)
}

// --- Update colon enters command mode ---

func TestUpdate_Colon_EntersCommandMode(t *testing.T) {
	t.Parallel()

	model := newInspectModel(indexing.NewInvertedIndex())
	result, _ := model.Update(tea.KeyPressMsg{Text: ":"})
	assert.Equal(t, inspectCommandModeCommand, result.(inspectModel).commandMode)
}

func TestUpdate_RealtimeRefreshMsg_InLogsMode_PreservesMode(t *testing.T) {
	t.Parallel()

	model := newInspectModel(indexing.NewInvertedIndex())
	model.mode = inspectViewModeLogs
	result, _ := model.Update(inspectRealtimeRefreshMsg{})
	assert.Equal(t, inspectViewModeLogs, result.(inspectModel).mode)
}

// --- updateInspectDocumentsSelection boundary cases ---

func TestUpdateInspectDocumentsSelection_Up_AtZero_IsNoop(t *testing.T) {
	t.Parallel()

	docs := make([]inspectDocumentRow, 5)
	model := inspectModel{filteredDocuments: docs, documentSelected: 0}
	model = updateInspectDocumentsSelection(model, "up")
	assert.Equal(t, 0, model.documentSelected)
}

func TestUpdateInspectDocumentsSelection_K_Decrements(t *testing.T) {
	t.Parallel()

	docs := make([]inspectDocumentRow, 5)
	model := inspectModel{filteredDocuments: docs, documentSelected: 3}
	model = updateInspectDocumentsSelection(model, "k")
	assert.Equal(t, 2, model.documentSelected)
}

func TestUpdateInspectDocumentsSelection_Down_AtMax_IsNoop(t *testing.T) {
	t.Parallel()

	docs := make([]inspectDocumentRow, 5)
	model := inspectModel{filteredDocuments: docs, documentSelected: 4}
	model = updateInspectDocumentsSelection(model, "down")
	assert.Equal(t, 4, model.documentSelected)
}

func TestUpdateInspectDocumentsSelection_J_Increments(t *testing.T) {
	t.Parallel()

	docs := make([]inspectDocumentRow, 5)
	model := inspectModel{filteredDocuments: docs, documentSelected: 1}
	model = updateInspectDocumentsSelection(model, "j")
	assert.Equal(t, 2, model.documentSelected)
}

// --- updateInspectCommandInputMode ---

func TestUpdateInspectCommandInputMode_Backspace_TrimsQuery(t *testing.T) {
	t.Parallel()

	model := inspectModel{commandMode: inspectCommandModeCommand, commandQuery: "ind"}
	result, _ := updateInspectCommandInputMode(model, tea.KeyPressMsg{Code: tea.KeyBackspace})
	assert.Equal(t, "in", result.(inspectModel).commandQuery)
}

func TestUpdateInspectCommandInputMode_Enter_UnknownCommand_SetsError(t *testing.T) {
	t.Parallel()

	model := inspectModel{commandMode: inspectCommandModeCommand, commandQuery: "bad"}
	result, _ := updateInspectCommandInputMode(model, tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.NotEmpty(t, result.(inspectModel).commandError)
}

func TestUpdateInspectCommandInputMode_Tab_Autocompletes(t *testing.T) {
	t.Parallel()

	model := inspectModel{commandMode: inspectCommandModeCommand, commandQuery: ":tl"}
	result, _ := updateInspectCommandInputMode(model, tea.KeyPressMsg{Code: tea.KeyTab})
	assert.Equal(t, "tlog", result.(inspectModel).commandQuery)
}

func TestUpdateInspectCommandInputMode_KeyRunes_AppendsChars(t *testing.T) {
	t.Parallel()

	model := inspectModel{commandMode: inspectCommandModeCommand, commandQuery: "in"}
	result, _ := updateInspectCommandInputMode(model, tea.KeyPressMsg{Text: "d"})
	assert.Equal(t, "ind", result.(inspectModel).commandQuery)
}

// --- updateInspectDocumentSearchMode delete key ---

func TestUpdateInspectDocumentSearchMode_Delete_Trims(t *testing.T) {
	t.Parallel()

	model := newDocSearchModel()
	model.documentSearchQuery = "srv"
	result, _ := updateInspectDocumentSearchMode(model, tea.KeyPressMsg{Code: tea.KeyDelete})
	assert.Equal(t, "sr", result.(inspectModel).documentSearchQuery)
}
