package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"idx/internal/features/indexing"
)

func TestView_DirectoriesMode_ReturnsNonEmpty(t *testing.T) {
	t.Parallel()

	// Arrange
	index := indexing.NewInvertedIndex()
	index.AddDocument("/repo/internal::service.go", "internal/service.go", 10)
	model := newInspectModel(index)
	model.height = 24
	model.width = 80

	// Act
	view := model.View().Content

	// Assert
	assert.NotEmpty(t, view)
}

func TestView_DocumentsMode_ContainsDocumentName(t *testing.T) {
	t.Parallel()

	// Arrange
	index := indexing.NewInvertedIndex()
	index.AddDocument("/repo/internal::service.go", "internal/service.go", 10)
	model := newInspectModel(index)
	model.mode = inspectViewModeDocuments
	model.height = 24
	model.width = 80
	// Manually set up documents since we're not going through update
	model.activeDirectory = "/repo/internal"
	model.documents = []inspectDocumentRow{
		{name: "service.go", path: "internal/service.go", length: 10, termCount: 5},
	}
	model.filteredDocuments = append([]inspectDocumentRow(nil), model.documents...)
	model.documentSelected = 0

	// Act
	view := model.View().Content

	// Assert
	assert.NotEmpty(t, view)
	assert.Contains(t, view, "service.go")
}

func TestView_LogsMode_ReturnsNonEmpty(t *testing.T) {
	t.Parallel()

	// Arrange
	index := indexing.NewInvertedIndex()
	index.AddDocument("/repo/internal::service.go", "internal/service.go", 10)
	model := newInspectModel(index)
	model.mode = inspectViewModeLogs
	model.height = 24
	model.width = 80
	model.logs = []inspectLogRow{{indexedAt: "2026-05-01T00:00:00Z", path: "/repo", hash: "abc"}}
	model.filteredLogs = model.logs

	// Act
	view := model.View().Content

	// Assert
	assert.NotEmpty(t, view)
}

func TestView_JSONMode_ReturnsNonEmpty(t *testing.T) {
	t.Parallel()

	// Arrange
	index := indexing.NewInvertedIndex()
	index.AddDocument("/repo/internal::service.go", "internal/service.go", 10)
	model := newInspectModel(index)
	model.mode = inspectViewModeJSON
	model.height = 24
	model.width = 80
	model.jsonLines = []string{`{"key": "value"}`, `}`}

	// Act
	view := model.View().Content

	// Assert
	assert.NotEmpty(t, view)
}

func TestView_Quitting_ReturnsNonEmpty(t *testing.T) {
	t.Parallel()

	// Arrange
	model := newInspectModel(indexing.NewInvertedIndex())
	model.quitting = true

	// Act
	view := model.View().Content

	// Assert
	assert.NotEmpty(t, view)
}

func TestInit_ReturnsNonNilCmd(t *testing.T) {
	t.Parallel()

	// Arrange
	model := newInspectModel(indexing.NewInvertedIndex())

	// Act
	cmd := model.Init()

	// Assert
	require.NotNil(t, cmd)
}

func TestRefreshInspectLogs_ReturnsValidModel(t *testing.T) {
	t.Parallel()

	// Arrange
	model := newInspectModel(indexing.NewInvertedIndex())
	model.logs = []inspectLogRow{}
	model.filteredLogs = []inspectLogRow{}

	// Act — even with absent log directory it must return without panicking
	_ = refreshInspectLogs(model)
}

func TestRunInspectTUI_UsesTestHook(t *testing.T) {
	t.Parallel()

	// Arrange
	called := false
	SetRunInspectTUITestHook(func(_ *indexing.InvertedIndex) error {
		called = true
		return nil
	})
	defer SetRunInspectTUITestHook(nil)

	// Act
	err := runInspectTUI(indexing.NewInvertedIndex())

	// Assert
	require.NoError(t, err)
	assert.True(t, called)
}

func TestSetRunInspectTUITestHook_NilRestoresDefault(t *testing.T) {
	t.Parallel()

	// Arrange
	SetRunInspectTUITestHook(func(_ *indexing.InvertedIndex) error { return nil })

	// Act
	SetRunInspectTUITestHook(nil)

	// Assert
	assert.NotNil(t, RunInspectTUITestHook())
}

func TestInspectLogsView_ReturnsNonEmptyString(t *testing.T) {
	t.Parallel()

	// Arrange
	index := indexing.NewInvertedIndex()
	index.AddDocument("/repo/internal::service.go", "internal/service.go", 10)
	model := newInspectModel(index)
	model.height = 24
	model.width = 80
	model.logs = []inspectLogRow{{indexedAt: "2026-05-01T00:00:00Z", path: "/repo", hash: "abc"}}
	model.filteredLogs = model.logs

	// Act
	result := inspectLogsView(model)

	// Assert
	assert.NotEmpty(t, result)
}

func TestInspectInputLine_NormalMode_DoesNotPanic(t *testing.T) {
	t.Parallel()

	// Arrange
	model := newInspectModel(indexing.NewInvertedIndex())
	model.width = 80

	// Act — verify no panic
	_ = inspectInputLine(model, false, "")
}

func TestInspectInputLine_SearchMode_DoesNotPanic(t *testing.T) {
	t.Parallel()

	// Arrange
	model := newInspectModel(indexing.NewInvertedIndex())
	model.width = 80

	// Act — verify no panic
	_ = inspectInputLine(model, true, "search term")
}

func TestInspectHorizontalWindow_NarrowWidth_DoesNotPanic(t *testing.T) {
	t.Parallel()

	// Act — verify no panic
	_ = inspectHorizontalWindow("hello world long line", 5, 0)
}

func TestInspectHorizontalWindow_WithOffset_DoesNotPanic(t *testing.T) {
	t.Parallel()

	// Act — verify no panic
	_ = inspectHorizontalWindow("hello world", 5, 3)
}

func TestInspectHighlightJSONLine_VariousInputs_DoNotPanic(t *testing.T) {
	t.Parallel()

	// Arrange
	lines := []string{
		`  "name": "service.go"`,
		`  "count": 42`,
		`  "flag": true`,
		`  "empty": null`,
		`  {},[],:`,
		``,
	}

	// Act — verify no panic for any input
	for _, line := range lines {
		_ = inspectHighlightJSONLine(line)
	}
}

func TestUpdateInspectLogsSelection_Up_DecrementsSelection(t *testing.T) {
	t.Parallel()

	// Arrange
	model := newInspectModel(indexing.NewInvertedIndex())
	model.logSelected = 5
	model.filteredLogs = make([]inspectLogRow, 10)

	// Act
	result := updateInspectLogsSelection(model, "up")

	// Assert
	assert.Equal(t, 4, result.logSelected)
}

func TestUpdateInspectLogsSelection_Up_DoesNotGoBelowZero(t *testing.T) {
	t.Parallel()

	// Arrange
	model := newInspectModel(indexing.NewInvertedIndex())
	model.logSelected = 0
	model.filteredLogs = make([]inspectLogRow, 10)

	// Act
	result := updateInspectLogsSelection(model, "up")

	// Assert
	assert.Equal(t, 0, result.logSelected)
}

func TestUpdateInspectLogsSelection_K_DecrementsSelection(t *testing.T) {
	t.Parallel()

	// Arrange
	model := newInspectModel(indexing.NewInvertedIndex())
	model.logSelected = 5
	model.filteredLogs = make([]inspectLogRow, 10)

	// Act
	result := updateInspectLogsSelection(model, "k")

	// Assert
	assert.Equal(t, 4, result.logSelected)
}

func TestUpdateInspectLogsSelection_Down_IncrementsSelection(t *testing.T) {
	t.Parallel()

	// Arrange
	model := newInspectModel(indexing.NewInvertedIndex())
	model.logSelected = 5
	model.filteredLogs = make([]inspectLogRow, 10)

	// Act
	result := updateInspectLogsSelection(model, "down")

	// Assert
	assert.Equal(t, 6, result.logSelected)
}

func TestUpdateInspectLogsSelection_Down_DoesNotExceedBounds(t *testing.T) {
	t.Parallel()

	// Arrange
	model := newInspectModel(indexing.NewInvertedIndex())
	model.logSelected = 9
	model.filteredLogs = make([]inspectLogRow, 10)

	// Act
	result := updateInspectLogsSelection(model, "down")

	// Assert
	assert.Equal(t, 9, result.logSelected)
}

func TestUpdateInspectLogsSelection_PageUp_ChangesSelection(t *testing.T) {
	t.Parallel()

	// Arrange
	model := newInspectModel(indexing.NewInvertedIndex())
	model.logSelected = 20
	model.height = 24
	model.filteredLogs = make([]inspectLogRow, 100)

	// Act
	result := updateInspectLogsSelection(model, "pgup")

	// Assert
	assert.Less(t, result.logSelected, 20)
}

func TestUpdateInspectLogsSelection_PageDown_ChangesSelection(t *testing.T) {
	t.Parallel()

	// Arrange
	model := newInspectModel(indexing.NewInvertedIndex())
	model.logSelected = 10
	model.height = 24
	model.filteredLogs = make([]inspectLogRow, 100)

	// Act
	result := updateInspectLogsSelection(model, "pgdown")

	// Assert
	assert.Greater(t, result.logSelected, 10)
}

func TestHandleInspectLogsViewAction_Slash_EntersSearchMode(t *testing.T) {
	t.Parallel()

	// Arrange
	model := newInspectModel(indexing.NewInvertedIndex())
	model.logSearchMode = false
	model.commandMode = inspectCommandModeNone

	// Act
	result, handled := handleInspectLogsViewAction(model, "/")

	// Assert
	require.True(t, handled)
	assert.True(t, result.logSearchMode)
	assert.Empty(t, result.logSearchQuery)
}

func TestHandleInspectLogsViewAction_Enter_ReturnsHandled(t *testing.T) {
	t.Parallel()

	// Arrange
	model := newInspectModel(indexing.NewInvertedIndex())

	// Act
	_, handled := handleInspectLogsViewAction(model, "enter")

	// Assert
	assert.True(t, handled)
}

func TestHandleInspectLogsViewAction_UnknownKey_NotHandled(t *testing.T) {
	t.Parallel()

	// Arrange
	model := newInspectModel(indexing.NewInvertedIndex())

	// Act
	_, handled := handleInspectLogsViewAction(model, "unknown-key")

	// Assert
	assert.False(t, handled)
}
