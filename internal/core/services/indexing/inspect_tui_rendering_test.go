package indexing

import (
	"strings"
	"testing"

	"idx/internal/core/domain"
)

func TestViewReturnsNonEmptyInDirectoriesMode(t *testing.T) {
	index := domain.NewInvertedIndex()
	index.AddDocument("/repo/internal::service.go", "internal/service.go", 10)
	model := newInspectModel(index)
	model.height = 24
	model.width = 80

	view := model.View().Content
	if view == "" {
		t.Fatal("expected non-empty view in directories mode")
	}
}

func TestViewReturnsNonEmptyInDocumentsMode(t *testing.T) {
	index := domain.NewInvertedIndex()
	index.AddDocument("/repo/internal::service.go", "internal/service.go", 10)
	model := newInspectModel(index)
	model.mode = inspectViewModeDocuments
	model.height = 24
	model.width = 80

	// Manually set up documents for this test since we're not going through update
	model.activeDirectory = "/repo/internal"
	model.documents = []inspectDocumentRow{
		{name: "service.go", path: "internal/service.go", length: 10, termCount: 5},
	}
	model.filteredDocuments = append([]inspectDocumentRow(nil), model.documents...)
	model.documentSelected = 0

	view := model.View().Content
	if view == "" {
		t.Fatal("expected non-empty view in documents mode")
	}
	if !strings.Contains(view, "service.go") {
		t.Fatalf("expected view to contain document name, got: %s", view)
	}
}

func TestViewReturnsNonEmptyInLogsMode(t *testing.T) {
	index := domain.NewInvertedIndex()
	index.AddDocument("/repo/internal::service.go", "internal/service.go", 10)
	model := newInspectModel(index)
	model.mode = inspectViewModeLogs
	model.height = 24
	model.width = 80
	model.logs = []inspectLogRow{{indexedAt: "2026-05-01T00:00:00Z", path: "/repo", hash: "abc"}}
	model.filteredLogs = model.logs

	view := model.View().Content
	if view == "" {
		t.Fatal("expected non-empty view in logs mode")
	}
}

func TestViewReturnsNonEmptyInJSONMode(t *testing.T) {
	index := domain.NewInvertedIndex()
	index.AddDocument("/repo/internal::service.go", "internal/service.go", 10)
	model := newInspectModel(index)
	model.mode = inspectViewModeJSON
	model.height = 24
	model.width = 80
	model.jsonLines = []string{`{"key": "value"}`, `}`}

	view := model.View().Content
	if view == "" {
		t.Fatal("expected non-empty view in JSON mode")
	}
}

func TestViewReturnsQuitMessageWhenQuitting(t *testing.T) {
	model := newInspectModel(domain.NewInvertedIndex())
	model.quitting = true

	view := model.View().Content
	if view == "" {
		t.Fatal("expected non-empty quit view")
	}
}

func TestInitReturnsNonNilCmd(t *testing.T) {
	model := newInspectModel(domain.NewInvertedIndex())
	cmd := model.Init()
	if cmd == nil {
		t.Fatal("expected Init to return a non-nil tea.Cmd")
	}
}

func TestRefreshInspectLogsReturnsModel(t *testing.T) {
	// refreshInspectLogs reads from the filesystem (current directory);
	// even with an empty or absent log directory it must return a valid model.
	model := newInspectModel(domain.NewInvertedIndex())
	model.logs = []inspectLogRow{}
	model.filteredLogs = []inspectLogRow{}

	refreshed := refreshInspectLogs(model)
	// Must return a valid inspectModel without panicking.
	_ = refreshed
}

func TestRunInspectUIUsesTestHook(t *testing.T) {
	called := false
	SetRunInspectTUITestHook(func(_ *domain.InvertedIndex) error {
		called = true
		return nil
	})
	defer SetRunInspectTUITestHook(nil)

	if err := RunInspectUI(domain.NewInvertedIndex()); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !called {
		t.Fatal("expected test hook to be invoked by RunInspectUI")
	}
}

func TestSetRunInspectTUITestHookNilRestoresDefault(t *testing.T) {
	// Replace hook then restore it; the restored function must not be nil.
	SetRunInspectTUITestHook(func(_ *domain.InvertedIndex) error { return nil })
	SetRunInspectTUITestHook(nil)
	hook := RunInspectTUITestHook()
	if hook == nil {
		t.Fatal("expected non-nil default hook after SetRunInspectTUITestHook(nil)")
	}
}

func TestInspectLogsViewReturnsNonEmptyString(t *testing.T) {
	index := domain.NewInvertedIndex()
	index.AddDocument("/repo/internal::service.go", "internal/service.go", 10)
	model := newInspectModel(index)
	model.height = 24
	model.width = 80
	model.logs = []inspectLogRow{{indexedAt: "2026-05-01T00:00:00Z", path: "/repo", hash: "abc"}}
	model.filteredLogs = model.logs

	result := inspectLogsView(model)
	if result == "" {
		t.Fatal("expected non-empty result from inspectLogsView")
	}
}

func TestInspectInputLineNormalMode(t *testing.T) {
	model := newInspectModel(domain.NewInvertedIndex())
	model.width = 80
	result := inspectInputLine(model, false, "")
	_ = result // just verify no panic
}

func TestInspectInputLineSearchMode(t *testing.T) {
	model := newInspectModel(domain.NewInvertedIndex())
	model.width = 80
	result := inspectInputLine(model, true, "search term")
	_ = result
}

func TestInspectHorizontalWindowNarrowWidth(t *testing.T) {
	result := inspectHorizontalWindow("hello world long line", 5, 0)
	_ = result // verify no panic
}

func TestInspectHorizontalWindowWithOffset(t *testing.T) {
	result := inspectHorizontalWindow("hello world", 5, 3)
	_ = result
}

func TestInspectHighlightJSONLineDoesNotPanic(t *testing.T) {
	lines := []string{
		`  "name": "service.go"`,
		`  "count": 42`,
		`  "flag": true`,
		`  "empty": null`,
		`  {},[],:`,
		``,
	}

	for _, line := range lines {
		result := inspectHighlightJSONLine(line)
		_ = result // just confirm no panic
	}
}

func TestUpdateInspectLogsSelectionUpDecrementsSelection(t *testing.T) {
	model := newInspectModel(domain.NewInvertedIndex())
	model.logSelected = 5
	model.filteredLogs = make([]inspectLogRow, 10)

	result := updateInspectLogsSelection(model, "up")
	if result.logSelected != 4 {
		t.Fatalf("expected selection 4, got %d", result.logSelected)
	}
}

func TestUpdateInspectLogsSelectionUpDoesNotGoBelowZero(t *testing.T) {
	model := newInspectModel(domain.NewInvertedIndex())
	model.logSelected = 0
	model.filteredLogs = make([]inspectLogRow, 10)

	result := updateInspectLogsSelection(model, "up")
	if result.logSelected != 0 {
		t.Fatalf("expected selection to stay at 0, got %d", result.logSelected)
	}
}

func TestUpdateInspectLogsSelectionKDecrementsSelection(t *testing.T) {
	model := newInspectModel(domain.NewInvertedIndex())
	model.logSelected = 5
	model.filteredLogs = make([]inspectLogRow, 10)

	result := updateInspectLogsSelection(model, "k")
	if result.logSelected != 4 {
		t.Fatalf("expected selection 4, got %d", result.logSelected)
	}
}

func TestUpdateInspectLogsSelectionDownIncrementsSelection(t *testing.T) {
	model := newInspectModel(domain.NewInvertedIndex())
	model.logSelected = 5
	model.filteredLogs = make([]inspectLogRow, 10)

	result := updateInspectLogsSelection(model, "down")
	if result.logSelected != 6 {
		t.Fatalf("expected selection 6, got %d", result.logSelected)
	}
}

func TestUpdateInspectLogsSelectionDownDoesNotExceedBounds(t *testing.T) {
	model := newInspectModel(domain.NewInvertedIndex())
	model.logSelected = 9
	model.filteredLogs = make([]inspectLogRow, 10)

	result := updateInspectLogsSelection(model, "down")
	if result.logSelected != 9 {
		t.Fatalf("expected selection to stay at 9, got %d", result.logSelected)
	}
}

func TestUpdateInspectLogsSelectionPageUpChangesSelection(t *testing.T) {
	model := newInspectModel(domain.NewInvertedIndex())
	model.logSelected = 20
	model.height = 24
	model.filteredLogs = make([]inspectLogRow, 100)

	result := updateInspectLogsSelection(model, "pgup")
	if result.logSelected >= 20 {
		t.Fatalf("expected selection to decrease from 20, got %d", result.logSelected)
	}
}

func TestUpdateInspectLogsSelectionPageDownChangesSelection(t *testing.T) {
	model := newInspectModel(domain.NewInvertedIndex())
	model.logSelected = 10
	model.height = 24
	model.filteredLogs = make([]inspectLogRow, 100)

	result := updateInspectLogsSelection(model, "pgdown")
	if result.logSelected <= 10 {
		t.Fatalf("expected selection to increase from 10, got %d", result.logSelected)
	}
}

func TestHandleInspectLogsViewActionSlashEntersSearchMode(t *testing.T) {
	model := newInspectModel(domain.NewInvertedIndex())
	model.logSearchMode = false
	model.commandMode = inspectCommandModeNone

	result, handled := handleInspectLogsViewAction(model, "/")
	if !handled {
		t.Fatal("expected '/' to be handled")
	}
	if !result.logSearchMode {
		t.Fatal("expected search mode to be enabled")
	}
	if result.logSearchQuery != "" {
		t.Fatalf("expected empty search query, got %q", result.logSearchQuery)
	}
}

func TestHandleInspectLogsViewActionEnterReturnsTrue(t *testing.T) {
	model := newInspectModel(domain.NewInvertedIndex())
	_, handled := handleInspectLogsViewAction(model, "enter")
	if !handled {
		t.Fatal("expected 'enter' to be handled")
	}
}

func TestHandleInspectLogsViewActionUnknownKeyReturnsFalse(t *testing.T) {
	model := newInspectModel(domain.NewInvertedIndex())
	_, handled := handleInspectLogsViewAction(model, "unknown-key")
	if handled {
		t.Fatal("expected unknown key to not be handled")
	}
}
