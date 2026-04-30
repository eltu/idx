package indexing

import (
	"os"
	"path/filepath"
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

func TestInspectEnterOpensDirectoryThenJSONAndEscReturnsToDocuments(t *testing.T) {
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

func TestInspectDocumentSlashSearchEscIgnoredAndEnterFinishes(t *testing.T) {
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
	if !updatedModel.documentSearchMode {
		t.Fatal("expected document search mode to remain enabled after esc")
	}
	if len(updatedModel.filteredDocuments) != 1 {
		t.Fatalf("expected filter to remain after esc, got %d results", len(updatedModel.filteredDocuments))
	}

	updated, _ = updatedModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updatedModel = updated.(inspectModel)
	if updatedModel.documentSearchMode {
		t.Fatal("expected document search mode disabled after enter")
	}
}

func TestInspectDocumentsEscReturnsToDirectories(t *testing.T) {
	model := inspectModel{
		mode: inspectViewModeDocuments,
		documents: []inspectDocumentRow{
			{name: "main", path: "cmd/idx/main.go"},
		},
		filteredDocuments: []inspectDocumentRow{
			{name: "main", path: "cmd/idx/main.go"},
		},
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updatedModel := updated.(inspectModel)
	if updatedModel.mode != inspectViewModeDirectories {
		t.Fatalf("expected directories mode after esc from documents, got %v", updatedModel.mode)
	}
}

func TestInspectLogsEscDoesNotLeaveMode(t *testing.T) {
	model := inspectModel{
		mode:         inspectViewModeLogs,
		logs:         []inspectLogRow{{indexedAt: "2026-04-30T10:00:00Z", path: "/repo", hash: "abc123", summary: "ok"}},
		filteredLogs: []inspectLogRow{{indexedAt: "2026-04-30T10:00:00Z", path: "/repo", hash: "abc123", summary: "ok"}},
		logSelected:  0,
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updatedModel := updated.(inspectModel)
	if updatedModel.mode != inspectViewModeLogs {
		t.Fatalf("expected logs mode to remain after esc, got %v", updatedModel.mode)
	}
}

func TestInspectCommandModeTabAutocompleteSingleMatch(t *testing.T) {
	model := inspectModel{mode: inspectViewModeDirectories}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	updatedModel := updated.(inspectModel)
	updated, _ = updatedModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tl")})
	updatedModel = updated.(inspectModel)
	updated, _ = updatedModel.Update(tea.KeyMsg{Type: tea.KeyTab})
	updatedModel = updated.(inspectModel)

	if updatedModel.commandQuery != "tlog" {
		t.Fatalf("expected command query to autocomplete to tlog, got %q", updatedModel.commandQuery)
	}
}

func TestInspectJSONModeAllowsCommandInput(t *testing.T) {
	model := inspectModel{
		mode:      inspectViewModeJSON,
		jsonLines: []string{"{}"},
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	updatedModel := updated.(inspectModel)
	if updatedModel.commandMode != inspectCommandModeCommand {
		t.Fatal("expected command mode to be enabled from JSON mode")
	}
}

func TestInspectCommandModeSwitchesToLogsNavigator(t *testing.T) {
	model := newInspectModel(domain.NewInvertedIndex())

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	updatedModel := updated.(inspectModel)
	if updatedModel.commandMode != inspectCommandModeCommand {
		t.Fatal("expected command mode enabled after colon")
	}

	updated, _ = updatedModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tlog")})
	updatedModel = updated.(inspectModel)
	updated, _ = updatedModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updatedModel = updated.(inspectModel)

	if updatedModel.mode != inspectViewModeLogs {
		t.Fatalf("expected logs mode after :tlog, got %v", updatedModel.mode)
	}
}

func TestInspectLogsEnterDoesNothing(t *testing.T) {
	model := inspectModel{
		mode:         inspectViewModeLogs,
		logs:         []inspectLogRow{{indexedAt: "2026-04-30T10:00:00Z", path: "/repo", hash: "abc123", summary: "ok"}},
		filteredLogs: []inspectLogRow{{indexedAt: "2026-04-30T10:00:00Z", path: "/repo", hash: "abc123", summary: "ok"}},
		logSelected:  0,
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updatedModel := updated.(inspectModel)
	if updatedModel.mode != inspectViewModeLogs {
		t.Fatalf("expected logs mode to remain after enter, got %v", updatedModel.mode)
	}
}

func TestInspectCommandModeSwitchesToIndexNavigator(t *testing.T) {
	model := inspectModel{mode: inspectViewModeLogs}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	updatedModel := updated.(inspectModel)
	updated, _ = updatedModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("index")})
	updatedModel = updated.(inspectModel)
	updated, _ = updatedModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updatedModel = updated.(inspectModel)

	if updatedModel.mode != inspectViewModeDirectories {
		t.Fatalf("expected directories mode after :index, got %v", updatedModel.mode)
	}
}

func TestInspectCommandModeUnknownCommandSetsError(t *testing.T) {
	model := newInspectModel(domain.NewInvertedIndex())

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	updatedModel := updated.(inspectModel)
	updated, _ = updatedModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("invalid")})
	updatedModel = updated.(inspectModel)
	updated, _ = updatedModel.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updatedModel = updated.(inspectModel)

	if updatedModel.commandError == "" {
		t.Fatal("expected command error for unknown command")
	}
}

func TestInspectLogTableUsesFixedSeparators(t *testing.T) {
	header := inspectLogTableHeader()
	if !strings.Contains(header, " | ") {
		t.Fatal("expected table header with fixed separators")
	}

	row := inspectLogTableRow(inspectLogRow{indexedAt: "2026-04-30T10:00:00Z", path: "/repo", hash: "abc123"})
	if !strings.Contains(row, " | ") {
		t.Fatal("expected table row with fixed separators")
	}
}

func TestInspectLogsHorizontalNavigationWithArrows(t *testing.T) {
	model := inspectModel{
		mode:         inspectViewModeLogs,
		width:        20,
		logs:         []inspectLogRow{{indexedAt: "2026-04-30T10:00:00Z", path: "/a/very/long/path", hash: "1234567890abcdef", summary: "summary"}},
		filteredLogs: []inspectLogRow{{indexedAt: "2026-04-30T10:00:00Z", path: "/a/very/long/path", hash: "1234567890abcdef", summary: "summary"}},
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRight})
	updatedModel := updated.(inspectModel)
	if updatedModel.logColumnOffset <= 0 {
		t.Fatalf("expected positive column offset after right key, got %d", updatedModel.logColumnOffset)
	}

	updated, _ = updatedModel.Update(tea.KeyMsg{Type: tea.KeyLeft})
	updatedModel = updated.(inspectModel)
	if updatedModel.logColumnOffset != 0 {
		t.Fatalf("expected zero column offset after left key, got %d", updatedModel.logColumnOffset)
	}
}

func TestReplaceInspectLogsPreservesSelection(t *testing.T) {
	model := inspectModel{
		mode: inspectViewModeLogs,
		logs: []inspectLogRow{
			{indexedAt: "a", jsonRaw: `{"id":1}`},
			{indexedAt: "b", jsonRaw: `{"id":2}`},
		},
		filteredLogs: []inspectLogRow{
			{indexedAt: "a", jsonRaw: `{"id":1}`},
			{indexedAt: "b", jsonRaw: `{"id":2}`},
		},
		logSelected: 1,
	}

	updated := replaceInspectLogs(model, []inspectLogRow{
		{indexedAt: "b", jsonRaw: `{"id":2}`},
		{indexedAt: "c", jsonRaw: `{"id":3}`},
	})

	if len(updated.filteredLogs) != 2 {
		t.Fatalf("expected 2 filtered logs, got %d", len(updated.filteredLogs))
	}

	if updated.logSelected != 0 {
		t.Fatalf("expected selected log index 0 after refresh, got %d", updated.logSelected)
	}
}

func TestParseInspectSummaryFields(t *testing.T) {
	indexedAt, pathValue, hash := parseInspectSummaryFields("indexed_at=2026-04-30T10:00:00Z path=/repo/internal hash=abcdef")
	if indexedAt != "2026-04-30T10:00:00Z" {
		t.Fatalf("unexpected indexed_at: %q", indexedAt)
	}
	if pathValue != "/repo/internal" {
		t.Fatalf("unexpected path: %q", pathValue)
	}
	if hash != "abcdef" {
		t.Fatalf("unexpected hash: %q", hash)
	}
}

func TestSortInspectLogsNewestFirst(t *testing.T) {
	rows := []inspectLogRow{
		{indexedAt: "2026-04-29T10:00:00Z", path: "/repo/a"},
		{indexedAt: "2026-04-30T10:00:00Z", path: "/repo/b"},
		{indexedAt: "2026-04-28T10:00:00Z", path: "/repo/c"},
	}

	sortInspectLogsNewestFirst(rows)

	if rows[0].indexedAt != "2026-04-30T10:00:00Z" {
		t.Fatalf("expected first log to be newest, got %q", rows[0].indexedAt)
	}
	if rows[1].indexedAt != "2026-04-29T10:00:00Z" {
		t.Fatalf("expected second log to be middle timestamp, got %q", rows[1].indexedAt)
	}
	if rows[2].indexedAt != "2026-04-28T10:00:00Z" {
		t.Fatalf("expected third log to be oldest, got %q", rows[2].indexedAt)
	}
}

func TestSortInspectLogsNewestFirstKeepsParsableBeforeUnknown(t *testing.T) {
	rows := []inspectLogRow{
		{indexedAt: "-", path: "/repo/a"},
		{indexedAt: "2026-04-30T10:00:00Z", path: "/repo/b"},
		{indexedAt: "unknown", path: "/repo/c"},
	}

	sortInspectLogsNewestFirst(rows)

	if rows[0].indexedAt != "2026-04-30T10:00:00Z" {
		t.Fatalf("expected parsable timestamp first, got %q", rows[0].indexedAt)
	}
}

func TestDiscoverInspectTransactionLogFilesFindsAllDirectories(t *testing.T) {
	root := t.TempDir()

	paths := []string{
		filepath.Join(root, ".idx", "logs", "tlog.idx"),
		filepath.Join(root, "internal", ".idx", "logs", "tlog.idx"),
		filepath.Join(root, "cmd", "idx", "logs", "tlog.idx"),
	}
	for _, path := range paths {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("expected directory creation without error, got %v", err)
		}
		if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
			t.Fatalf("expected file creation without error, got %v", err)
		}
	}

	found, err := discoverInspectTransactionLogFiles(root)
	if err != nil {
		t.Fatalf("expected discovery without error, got %v", err)
	}

	if len(found) != 3 {
		t.Fatalf("expected 3 tlog files, got %d", len(found))
	}
}

func TestIsInspectTransactionLogPathMatchesSupportedLayouts(t *testing.T) {
	if !isInspectTransactionLogPath("/repo/.idx/logs/tlog.idx") {
		t.Fatal("expected .idx/logs/tlog.idx to be recognized")
	}

	if !isInspectTransactionLogPath("/repo/any/idx/logs/tlog.idx") {
		t.Fatal("expected idx/logs/tlog.idx to be recognized")
	}

	if isInspectTransactionLogPath("/repo/.idx/logs/other.idx") {
		t.Fatal("expected non-tlog file to be ignored")
	}
}
