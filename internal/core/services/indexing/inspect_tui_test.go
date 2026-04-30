package indexing

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"idx/internal/core/domain"
)

func TestAdjustInspectDirectoriesViewportKeepsSelectionVisible(t *testing.T) {
	model := inspectModel{
		directories:         make([]inspectDirectoryRow, 20),
		filteredDirectories: make([]inspectDirectoryRow, 20),
		directorySelected:   12,
		directoryStart:      0,
		height:              15,
	}

	model = adjustInspectDirectoriesViewport(model)

	start, end := inspectDirectoriesVisibleRange(model)
	if model.directorySelected < start || model.directorySelected >= end {
		t.Fatalf("expected selected index %d to be visible in [%d, %d)", model.directorySelected, start, end)
	}
}

func TestInspectDirectoriesVisibleRangeUsesViewportWindow(t *testing.T) {
	model := inspectModel{
		directories:         make([]inspectDirectoryRow, 10),
		filteredDirectories: make([]inspectDirectoryRow, 10),
		directorySelected:   6,
		directoryStart:      4,
		height:              13,
	}

	start, end := inspectDirectoriesVisibleRange(model)
	if start != 4 || end != 6 {
		t.Fatalf("expected visible range [4, 6), got [%d, %d)", start, end)
	}
}

func TestInspectTruncateLineRespectsWidth(t *testing.T) {
	line := inspectTruncateLine("123456789", 6)
	if line != "123..." {
		t.Fatalf("expected truncated line 123..., got %q", line)
	}
}

func TestInspectTruncateLineNoTruncationWhenFits(t *testing.T) {
	line := inspectTruncateLine("short", 10)
	if line != "short" {
		t.Fatalf("expected unmodified line, got %q", line)
	}
}

func TestInspectEnterOpensDirectoryThenJSONAndEscReturns(t *testing.T) {
	index := domain.NewInvertedIndex()
	index.AddDocument("/repo/internal::doc1", "internal/doc1.go", 8)
	index.AddTerm("alpha", "/repo/internal::doc1", 2, []int{1, 4})

	model := newInspectModel(index)
	if model.mode != inspectViewModeDirectories {
		t.Fatalf("expected directories mode initially, got %v", model.mode)
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	documentsModel := updated.(inspectModel)
	if documentsModel.mode != inspectViewModeDocuments {
		t.Fatalf("expected documents mode after opening directory, got %v", documentsModel.mode)
	}

	updated, _ = documentsModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	jsonModel := updated.(inspectModel)

	if jsonModel.mode != inspectViewModeJSON {
		t.Fatalf("expected JSON mode after enter, got %v", jsonModel.mode)
	}

	if len(jsonModel.jsonLines) == 0 {
		t.Fatal("expected JSON lines to be populated")
	}

	updated, _ = jsonModel.Update(tea.KeyMsg{Type: tea.KeyEsc})
	documentsModel = updated.(inspectModel)
	if documentsModel.mode != inspectViewModeDocuments {
		t.Fatalf("expected documents mode after esc from JSON, got %v", documentsModel.mode)
	}

	updated, _ = documentsModel.Update(tea.KeyMsg{Type: tea.KeyEsc})
	directoriesModel := updated.(inspectModel)
	if directoriesModel.mode != inspectViewModeDirectories {
		t.Fatalf("expected directories mode after esc from documents, got %v", directoriesModel.mode)
	}
}

func TestInspectDocumentJSONIncludesDocumentFieldsAndTerms(t *testing.T) {
	index := domain.NewInvertedIndex()
	index.AddDocument("doc1", "internal/doc1.go", 8)
	index.AddTerm("beta", "doc1", 1, []int{3})
	index.AddTerm("alpha", "doc1", 2, []int{1, 4})

	jsonText, err := inspectDocumentJSON(index, inspectDocumentRow{key: "doc1"})
	if err != nil {
		t.Fatalf("expected JSON without error, got %v", err)
	}

	expectedSnippets := []string{
		`"name": "doc1"`,
		`"path": "internal/doc1.go"`,
		`"uniqueTerms": 2`,
		`"term": "alpha"`,
		`"term": "beta"`,
	}

	for _, snippet := range expectedSnippets {
		if !strings.Contains(jsonText, snippet) {
			t.Fatalf("expected JSON to contain %q, got %s", snippet, jsonText)
		}
	}
}

func TestInspectJSONStringIsKey(t *testing.T) {
	line := `  "name": "value"`
	_, end := inspectReadJSONString(line, 2)
	if !inspectJSONStringIsKey(line, end) {
		t.Fatal("expected first JSON string to be detected as key")
	}
}

func TestInspectReadJSONKeyword(t *testing.T) {
	token, _, ok := inspectReadJSONKeyword("  true,", 2)
	if !ok || token != "true" {
		t.Fatalf("expected keyword true, got ok=%v token=%q", ok, token)
	}

	_, _, ok = inspectReadJSONKeyword("truename", 0)
	if ok {
		t.Fatal("expected no keyword match for word prefix")
	}
}

func TestInspectReadJSONNumber(t *testing.T) {
	if !inspectStartsJSONNumber("-12.5e+3", 0) {
		t.Fatal("expected numeric start detection")
	}

	token, end := inspectReadJSONNumber("-12.5e+3,", 0)
	if token != "-12.5e+3" || end != 8 {
		t.Fatalf("expected number -12.5e+3 ending at 8, got token=%q end=%d", token, end)
	}
}

func TestInspectListPgDownMovesByPage(t *testing.T) {
	model := inspectModel{
		directories:         make([]inspectDirectoryRow, 20),
		filteredDirectories: make([]inspectDirectoryRow, 20),
		directorySelected:   0,
		directoryStart:      0,
		height:              16,
		mode:                inspectViewModeDirectories,
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	directoriesModel := updated.(inspectModel)

	if directoriesModel.directorySelected != 4 {
		t.Fatalf("expected selected index 4 after pgdown, got %d", directoriesModel.directorySelected)
	}
}

func TestInspectListPgUpMovesByPage(t *testing.T) {
	model := inspectModel{
		documents:         make([]inspectDocumentRow, 20),
		filteredDocuments: make([]inspectDocumentRow, 20),
		documentSelected:  9,
		documentStart:     5,
		height:            16,
		mode:              inspectViewModeDocuments,
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	documentsModel := updated.(inspectModel)

	if documentsModel.documentSelected != 6 {
		t.Fatalf("expected selected index 6 after pgup, got %d", documentsModel.documentSelected)
	}
}

func TestInspectDirectorySlashSearchFiltersRows(t *testing.T) {
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

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	updatedModel := updated.(inspectModel)
	if !updatedModel.directorySearchMode {
		t.Fatal("expected directory search mode to be enabled after slash")
	}

	updated, _ = updatedModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d', 'o', 'c'}})
	updatedModel = updated.(inspectModel)
	if len(updatedModel.filteredDirectories) != 1 {
		t.Fatalf("expected one filtered directory, got %d", len(updatedModel.filteredDirectories))
	}
	if updatedModel.filteredDirectories[0].path != "/repo/docs" {
		t.Fatalf("expected /repo/docs after filtering, got %q", updatedModel.filteredDirectories[0].path)
	}
}

func TestInspectDocumentSlashSearchEscClearsFilter(t *testing.T) {
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

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	updatedModel := updated.(inspectModel)
	updated, _ = updatedModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s', 'e', 'a', 'r', 'c', 'h'}})
	updatedModel = updated.(inspectModel)
	if len(updatedModel.filteredDocuments) != 1 {
		t.Fatalf("expected one filtered document, got %d", len(updatedModel.filteredDocuments))
	}

	updated, _ = updatedModel.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updatedModel = updated.(inspectModel)
	if updatedModel.documentSearchMode {
		t.Fatal("expected document search mode disabled after esc")
	}
	if len(updatedModel.filteredDocuments) != 2 {
		t.Fatalf("expected full document list after esc, got %d", len(updatedModel.filteredDocuments))
	}
}
