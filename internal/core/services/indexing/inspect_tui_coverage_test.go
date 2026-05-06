package indexing

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"idx/internal/core/domain"
)

// --- clampInt ---

func TestClampIntBelowLowReturnsLow(t *testing.T) {
	if got := clampInt(-5, 0, 10); got != 0 {
		t.Fatalf("expected 0, got %d", got)
	}
}

func TestClampIntAboveHighReturnsHigh(t *testing.T) {
	if got := clampInt(15, 0, 10); got != 10 {
		t.Fatalf("expected 10, got %d", got)
	}
}

func TestClampIntWithinRangeReturnsValue(t *testing.T) {
	if got := clampInt(5, 0, 10); got != 5 {
		t.Fatalf("expected 5, got %d", got)
	}
}

// --- inspectInputLine ---

func TestInspectInputLineShowsCommandError(t *testing.T) {
	model := inspectModel{
		commandMode:  inspectCommandModeNone,
		commandError: "unknown command \"bad\"",
	}
	line := inspectInputLine(model, false, "")
	if !strings.Contains(line, "unknown command") {
		t.Fatalf("expected error message in line, got %q", line)
	}
}

func TestInspectInputLineShowsNonEmptySearchQuery(t *testing.T) {
	model := inspectModel{commandMode: inspectCommandModeNone}
	line := inspectInputLine(model, false, "myquery")
	if !strings.Contains(line, "myquery") {
		t.Fatalf("expected search query in line, got %q", line)
	}
}

func TestInspectInputLineCommandModeWithSuggestions(t *testing.T) {
	model := inspectModel{
		commandMode:  inspectCommandModeCommand,
		commandQuery: "ind",
	}
	line := inspectInputLine(model, false, "")
	if !strings.Contains(line, "ind") {
		t.Fatalf("expected command query in line, got %q", line)
	}
}

func TestInspectInputLineCommandModeNoSuggestions(t *testing.T) {
	model := inspectModel{
		commandMode:  inspectCommandModeCommand,
		commandQuery: "zzz",
	}
	line := inspectInputLine(model, false, "")
	if !strings.Contains(line, "zzz") {
		t.Fatalf("expected command query in line, got %q", line)
	}
}

// --- inspectHorizontalWindow ---

func TestInspectHorizontalWindowZeroWidthReturnsEmpty(t *testing.T) {
	got := inspectHorizontalWindow("hello", 0, 0)
	if got != "" {
		t.Fatalf("expected empty string for zero width, got %q", got)
	}
}

func TestInspectHorizontalWindowOffsetBeyondTextReturnsEmpty(t *testing.T) {
	got := inspectHorizontalWindow("hello", 5, 100)
	if got != "" {
		t.Fatalf("expected empty string for offset beyond text, got %q", got)
	}
}

func TestInspectHorizontalWindowNegativeOffsetTreatedAsZero(t *testing.T) {
	got := inspectHorizontalWindow("hello", 3, -1)
	if !strings.HasPrefix(got, "hel") {
		t.Fatalf("expected 'hel', got %q", got)
	}
}

// --- adjustInspectJSONViewport ---

func TestAdjustInspectJSONViewportClampsJsonStart(t *testing.T) {
	model := inspectModel{
		jsonLines: []string{"a", "b", "c", "d", "e"},
		jsonStart: 100,
		height:    10,
	}
	model = adjustInspectJSONViewport(model)
	if model.jsonStart < 0 {
		t.Fatalf("expected jsonStart >= 0, got %d", model.jsonStart)
	}
	if model.jsonStart > len(model.jsonLines) {
		t.Fatalf("expected jsonStart <= len(jsonLines), got %d", model.jsonStart)
	}
}

func TestAdjustInspectJSONViewportEmptyLines(t *testing.T) {
	model := inspectModel{jsonLines: nil, jsonStart: 5}
	model = adjustInspectJSONViewport(model)
	if model.jsonStart != 0 {
		t.Fatalf("expected jsonStart=0 for empty lines, got %d", model.jsonStart)
	}
}

// --- inspectJSONRange ---

func TestInspectJSONRangeStartAtOrBeyondEndClampsToLastLine(t *testing.T) {
	// jsonStart is so large that start >= end, should clamp to last line
	model := inspectModel{
		jsonLines: []string{"line1", "line2", "line3"},
		jsonStart: 999,
		height:    1, // inspectJSONHeight = max(1-7, 1) = 1
	}
	start, end := inspectJSONRange(model)
	if start >= end {
		t.Fatalf("expected start < end after clamp, got start=%d end=%d", start, end)
	}
}

// --- inspectReadJSONString ---

func TestInspectReadJSONStringEscapedQuote(t *testing.T) {
	line := `"hello \"world\""`
	token, end := inspectReadJSONString(line, 0)
	if !strings.HasPrefix(token, `"`) {
		t.Fatalf("expected token to start with quote, got %q", token)
	}
	_ = end
}

func TestInspectReadJSONStringUnterminated(t *testing.T) {
	line := `"unterminated`
	token, end := inspectReadJSONString(line, 0)
	if token == "" {
		t.Fatal("expected non-empty token for unterminated string")
	}
	if end != len(line) {
		t.Fatalf("expected end=%d for unterminated string, got %d", len(line), end)
	}
}

// --- adjustInspectDocumentsViewport ---

func TestAdjustInspectDocumentsViewportEmptyResetsSelection(t *testing.T) {
	model := inspectModel{
		filteredDocuments: nil,
		documentStart:     3,
		documentSelected:  5,
	}
	model = adjustInspectDocumentsViewport(model)
	if model.documentStart != 0 || model.documentSelected != 0 {
		t.Fatalf("expected (0,0) for empty docs, got (%d, %d)", model.documentStart, model.documentSelected)
	}
}

func TestAdjustInspectDocumentsViewportScrollsDown(t *testing.T) {
	docs := make([]inspectDocumentRow, 20)
	model := inspectModel{
		filteredDocuments: docs,
		documentSelected:  15,
		documentStart:     0,
		height:            10, // listHeight = max(10-8, 1) = 2
	}
	model = adjustInspectDocumentsViewport(model)
	if model.documentStart > model.documentSelected {
		t.Fatalf("expected documentStart <= documentSelected, got start=%d sel=%d", model.documentStart, model.documentSelected)
	}
}

func TestAdjustInspectDocumentsViewportScrollsUp(t *testing.T) {
	docs := make([]inspectDocumentRow, 20)
	model := inspectModel{
		filteredDocuments: docs,
		documentSelected:  2,
		documentStart:     10,
		height:            20,
	}
	model = adjustInspectDocumentsViewport(model)
	if model.documentStart > model.documentSelected {
		t.Fatalf("expected documentStart <= documentSelected, got start=%d sel=%d", model.documentStart, model.documentSelected)
	}
}

// --- updateInspectDocumentsSelection ---

func TestUpdateInspectDocumentsSelectionPgUp(t *testing.T) {
	docs := make([]inspectDocumentRow, 20)
	model := inspectModel{
		filteredDocuments: docs,
		documentSelected:  10,
		height:            10,
	}
	model = updateInspectDocumentsSelection(model, "pgup")
	if model.documentSelected >= 10 {
		t.Fatalf("expected documentSelected < 10 after pgup, got %d", model.documentSelected)
	}
}

func TestUpdateInspectDocumentsSelectionPgUpClampsToZero(t *testing.T) {
	docs := make([]inspectDocumentRow, 5)
	model := inspectModel{
		filteredDocuments: docs,
		documentSelected:  1,
		height:            20,
	}
	model = updateInspectDocumentsSelection(model, "pgup")
	if model.documentSelected < 0 {
		t.Fatalf("expected documentSelected >= 0 after pgup, got %d", model.documentSelected)
	}
}

func TestUpdateInspectDocumentsSelectionPgDown(t *testing.T) {
	docs := make([]inspectDocumentRow, 20)
	model := inspectModel{
		filteredDocuments: docs,
		documentSelected:  0,
		height:            10,
	}
	model = updateInspectDocumentsSelection(model, "pgdown")
	if model.documentSelected <= 0 {
		t.Fatalf("expected documentSelected > 0 after pgdown, got %d", model.documentSelected)
	}
}

func TestUpdateInspectDocumentsSelectionPgDownClampsToMax(t *testing.T) {
	docs := make([]inspectDocumentRow, 5)
	model := inspectModel{
		filteredDocuments: docs,
		documentSelected:  4,
		height:            20,
	}
	model = updateInspectDocumentsSelection(model, "pgdown")
	if model.documentSelected >= len(docs) {
		t.Fatalf("expected documentSelected < len(docs), got %d", model.documentSelected)
	}
}

// --- updateInspectDirectorySearchMode ---

func newDirectorySearchModel() inspectModel {
	return inspectModel{
		mode:                inspectViewModeDirectories,
		directorySearchMode: true,
		directories: []inspectDirectoryRow{
			{path: "/repo/internal"},
			{path: "/repo/cmd"},
		},
		filteredDirectories: []inspectDirectoryRow{
			{path: "/repo/internal"},
			{path: "/repo/cmd"},
		},
	}
}

func TestUpdateInspectDirectorySearchModeKeyRunes(t *testing.T) {
	model := newDirectorySearchModel()
	result, _ := updateInspectDirectorySearchMode(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("int")})
	updated := result.(inspectModel)
	if updated.directorySearchQuery != "int" {
		t.Fatalf("expected 'int', got %q", updated.directorySearchQuery)
	}
}

func TestUpdateInspectDirectorySearchModeEnterExitsSearch(t *testing.T) {
	model := newDirectorySearchModel()
	model.directorySearchQuery = "int"
	result, _ := updateInspectDirectorySearchMode(model, tea.KeyMsg{Type: tea.KeyEnter})
	updated := result.(inspectModel)
	if updated.directorySearchMode {
		t.Fatal("expected directorySearchMode=false after enter")
	}
}

func TestUpdateInspectDirectorySearchModeCtrlCQuits(t *testing.T) {
	model := newDirectorySearchModel()
	result, _ := updateInspectDirectorySearchMode(model, tea.KeyMsg{Type: tea.KeyCtrlC})
	updated := result.(inspectModel)
	if !updated.quitting {
		t.Fatal("expected quitting=true after ctrl+c")
	}
}

// --- updateInspectLogSearchMode ---

func newLogSearchModel() inspectModel {
	return inspectModel{
		mode:          inspectViewModeLogs,
		logSearchMode: true,
		logs: []inspectLogRow{
			{path: "/repo", indexedAt: "2026-05-01", hash: "abc"},
		},
		filteredLogs: []inspectLogRow{
			{path: "/repo", indexedAt: "2026-05-01", hash: "abc"},
		},
	}
}

func TestUpdateInspectLogSearchModeEnterExitsSearch(t *testing.T) {
	model := newLogSearchModel()
	model.logSearchQuery = "repo"
	result, _ := updateInspectLogSearchMode(model, tea.KeyMsg{Type: tea.KeyEnter})
	updated := result.(inspectModel)
	if updated.logSearchMode {
		t.Fatal("expected logSearchMode=false after enter")
	}
}

func TestUpdateInspectLogSearchModeKeyRunesAppendsQuery(t *testing.T) {
	model := newLogSearchModel()
	result, _ := updateInspectLogSearchMode(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("rep")})
	updated := result.(inspectModel)
	if updated.logSearchQuery != "rep" {
		t.Fatalf("expected 'rep', got %q", updated.logSearchQuery)
	}
}

func TestUpdateInspectLogSearchModeBackspaceTrims(t *testing.T) {
	model := newLogSearchModel()
	model.logSearchQuery = "repo"
	result, _ := updateInspectLogSearchMode(model, tea.KeyMsg{Type: tea.KeyBackspace})
	updated := result.(inspectModel)
	if updated.logSearchQuery != "rep" {
		t.Fatalf("expected 'rep' after backspace, got %q", updated.logSearchQuery)
	}
}

func TestUpdateInspectLogSearchModeCtrlCQuits(t *testing.T) {
	model := newLogSearchModel()
	result, _ := updateInspectLogSearchMode(model, tea.KeyMsg{Type: tea.KeyCtrlC})
	updated := result.(inspectModel)
	if !updated.quitting {
		t.Fatal("expected quitting=true after ctrl+c")
	}
}

// --- autocompleteInspectCommand multi-match path ---

func TestAutocompleteInspectCommandMultiMatchExpandsCommonPrefix(t *testing.T) {
	// Temporarily add a second command with a common prefix to trigger multi-match path.
	original := inspectAvailableCommands
	inspectAvailableCommands = []string{"inspect", "index"}
	defer func() { inspectAvailableCommands = original }()

	// "in" matches both "inspect" and "index", common prefix is "in"
	// len("in") == len("in"), so returns query
	result := autocompleteInspectCommand(":in")
	// Since common prefix "in" is not longer than normalized "in", it returns the original query
	if result != ":in" {
		t.Fatalf("expected ':in' when prefix not longer than query, got %q", result)
	}
}

func TestAutocompleteInspectCommandMultiMatchExtendsPrefix(t *testing.T) {
	// Temporarily override to have two commands where query is shorter than their common prefix.
	original := inspectAvailableCommands
	inspectAvailableCommands = []string{"inspect", "inspector"}
	defer func() { inspectAvailableCommands = original }()

	// "ins" matches both, common prefix is "inspect" which is longer than "ins"
	result := autocompleteInspectCommand(":ins")
	if result != "inspect" {
		t.Fatalf("expected 'inspect' as extended common prefix, got %q", result)
	}
}

func TestAutocompleteInspectCommandMultiMatchNoCommonPrefix(t *testing.T) {
	original := inspectAvailableCommands
	inspectAvailableCommands = []string{"alpha", "beta"}
	defer func() { inspectAvailableCommands = original }()

	// Empty normalized matches all, but normalized == "" causes early return
	// Try with a prefix that somehow matches both — impossible with alpha/beta
	// Instead test with two different starting chars which can't share prefix
	result := autocompleteInspectCommand(":a")
	// "a" only matches "alpha" → single match → returns "alpha"
	if result != "alpha" {
		t.Fatalf("expected 'alpha', got %q", result)
	}
}

// --- inspectDocumentJSON ---

func TestInspectDocumentJSONNilIndexReturnsError(t *testing.T) {
	row := inspectDocumentRow{key: "dir::file.go"}
	_, err := inspectDocumentJSON(nil, row)
	if err == nil {
		t.Fatal("expected error for nil index")
	}
}

func TestInspectDocumentJSONMissingDocumentReturnsError(t *testing.T) {
	index := domain.NewInvertedIndex()
	row := inspectDocumentRow{key: "dir::missing.go"}
	_, err := inspectDocumentJSON(index, row)
	if err == nil {
		t.Fatal("expected error for missing document")
	}
}

func TestInspectDocumentJSONReturnsValidJSON(t *testing.T) {
	index := domain.NewInvertedIndex()
	index.AddDocument("dir::file.go", "dir/file.go", 42)
	row := inspectDocumentRow{key: "dir::file.go", name: "file.go", path: "dir/file.go"}
	result, err := inspectDocumentJSON(index, row)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "file.go") {
		t.Fatalf("expected JSON to contain filename, got %q", result)
	}
}

// --- inspectJSONStringIsKey ---

func TestInspectJSONStringIsKeyReturnsFalseWhenNoColon(t *testing.T) {
	line := `"value" , "next"`
	// from=7 (after the closing quote of "value"), there's no colon
	if inspectJSONStringIsKey(line, 7) {
		t.Fatal("expected false when colon not found after string")
	}
}

func TestInspectJSONStringIsKeyReturnsFalseAtEnd(t *testing.T) {
	line := `"value"`
	if inspectJSONStringIsKey(line, len(line)) {
		t.Fatal("expected false when from >= len(line)")
	}
}

// --- replaceInspectLogs selection preservation ---

func TestReplaceInspectLogsPreservesSelectedRow(t *testing.T) {
	logs := []inspectLogRow{
		{path: "/a", jsonRaw: "raw-a"},
		{path: "/b", jsonRaw: "raw-b"},
	}
	model := inspectModel{
		logs:         logs,
		filteredLogs: logs,
		logSelected:  1,
	}
	newLogs := []inspectLogRow{
		{path: "/c", jsonRaw: "raw-c"},
		{path: "/b", jsonRaw: "raw-b"},
	}
	model = replaceInspectLogs(model, newLogs)
	// raw-b should still be selected
	if model.logSelected < 0 || model.logSelected >= len(model.filteredLogs) {
		t.Fatalf("logSelected out of range: %d (len=%d)", model.logSelected, len(model.filteredLogs))
	}
	if model.filteredLogs[model.logSelected].jsonRaw != "raw-b" {
		t.Fatalf("expected raw-b to remain selected, got %q", model.filteredLogs[model.logSelected].jsonRaw)
	}
}

// --- updateInspectJSONMode ---

func newJSONModel() inspectModel {
	// 30 lines ensures scrolling works with height=20 (inspectJSONHeight = max(20-7,1)=13)
	lines := make([]string, 30)
	for i := range lines {
		lines[i] = "line"
	}
	return inspectModel{
		mode:      inspectViewModeJSON,
		jsonLines: lines,
		jsonStart: 2,
		height:    20,
		width:     80,
	}
}

func TestUpdateInspectJSONModeQuitCtrlC(t *testing.T) {
	model := newJSONModel()
	result, _ := updateInspectJSONMode(model, tea.KeyMsg{Type: tea.KeyCtrlC})
	updated := result.(inspectModel)
	if !updated.quitting {
		t.Fatal("expected quitting after ctrl+c")
	}
}

func TestUpdateInspectJSONModeQuitQ(t *testing.T) {
	model := newJSONModel()
	result, _ := updateInspectJSONMode(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	updated := result.(inspectModel)
	if !updated.quitting {
		t.Fatal("expected quitting after q")
	}
}

func TestUpdateInspectJSONModeEscReturnsToDocuments(t *testing.T) {
	model := newJSONModel()
	model.jsonReturnMode = inspectViewModeDocuments
	result, _ := updateInspectJSONMode(model, tea.KeyMsg{Type: tea.KeyEsc})
	updated := result.(inspectModel)
	if updated.mode != inspectViewModeDocuments {
		t.Fatalf("expected documents mode after esc, got %v", updated.mode)
	}
}

func TestUpdateInspectJSONModeEscReturnsToLogs(t *testing.T) {
	model := newJSONModel()
	model.jsonReturnMode = inspectViewModeLogs
	result, _ := updateInspectJSONMode(model, tea.KeyMsg{Type: tea.KeyEsc})
	updated := result.(inspectModel)
	if updated.mode != inspectViewModeLogs {
		t.Fatalf("expected logs mode after esc, got %v", updated.mode)
	}
}

func TestUpdateInspectJSONModeUpDecrementsStart(t *testing.T) {
	model := newJSONModel()
	model.jsonStart = 3
	result, _ := updateInspectJSONMode(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	updated := result.(inspectModel)
	if updated.jsonStart >= 3 {
		t.Fatalf("expected jsonStart < 3 after k, got %d", updated.jsonStart)
	}
}

func TestUpdateInspectJSONModeDownIncrementsStart(t *testing.T) {
	model := newJSONModel()
	model.jsonStart = 0
	result, _ := updateInspectJSONMode(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	updated := result.(inspectModel)
	if updated.jsonStart <= 0 {
		t.Fatalf("expected jsonStart > 0 after j, got %d", updated.jsonStart)
	}
}

func TestUpdateInspectJSONModePgUpDecrementsStart(t *testing.T) {
	model := newJSONModel()
	model.jsonStart = 3
	result, _ := updateInspectJSONMode(model, tea.KeyMsg{Type: tea.KeyPgUp})
	updated := result.(inspectModel)
	if updated.jsonStart >= 3 {
		t.Fatalf("expected jsonStart < 3 after pgup, got %d", updated.jsonStart)
	}
}

func TestUpdateInspectJSONModePgDownIncrementsStart(t *testing.T) {
	model := newJSONModel()
	model.jsonStart = 0
	result, _ := updateInspectJSONMode(model, tea.KeyMsg{Type: tea.KeyPgDown})
	updated := result.(inspectModel)
	if updated.jsonStart <= 0 {
		t.Fatalf("expected jsonStart > 0 after pgdown, got %d", updated.jsonStart)
	}
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

func TestUpdateInspectDocumentSearchModeEnterExitsSearch(t *testing.T) {
	model := newDocSearchModel()
	model.documentSearchQuery = "srv"
	result, _ := updateInspectDocumentSearchMode(model, tea.KeyMsg{Type: tea.KeyEnter})
	updated := result.(inspectModel)
	if updated.documentSearchMode {
		t.Fatal("expected documentSearchMode=false after enter")
	}
}

func TestUpdateInspectDocumentSearchModeKeyRunesAppendsQuery(t *testing.T) {
	model := newDocSearchModel()
	result, _ := updateInspectDocumentSearchMode(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("srv")})
	updated := result.(inspectModel)
	if updated.documentSearchQuery != "srv" {
		t.Fatalf("expected 'srv', got %q", updated.documentSearchQuery)
	}
}

func TestUpdateInspectDocumentSearchModeCtrlCQuits(t *testing.T) {
	model := newDocSearchModel()
	result, _ := updateInspectDocumentSearchMode(model, tea.KeyMsg{Type: tea.KeyCtrlC})
	updated := result.(inspectModel)
	if !updated.quitting {
		t.Fatal("expected quitting=true after ctrl+c")
	}
}

// --- updateInspectDirectoriesMode navigation ---

func newDirectoriesModel() inspectModel {
	return inspectModel{
		mode: inspectViewModeDirectories,
		filteredDirectories: []inspectDirectoryRow{
			{path: "/repo/a"},
			{path: "/repo/b"},
			{path: "/repo/c"},
			{path: "/repo/d"},
			{path: "/repo/e"},
		},
		directorySelected: 2,
		height:            20,
		width:             80,
	}
}

func TestUpdateInspectDirectoriesModeQuits(t *testing.T) {
	model := newDirectoriesModel()
	result, _ := updateInspectDirectoriesMode(model, tea.KeyMsg{Type: tea.KeyCtrlC})
	updated := result.(inspectModel)
	if !updated.quitting {
		t.Fatal("expected quitting after ctrl+c")
	}
}

func TestUpdateInspectDirectoriesModePgUp(t *testing.T) {
	model := newDirectoriesModel()
	model.directorySelected = 4
	result, _ := updateInspectDirectoriesMode(model, tea.KeyMsg{Type: tea.KeyPgUp})
	updated := result.(inspectModel)
	if updated.directorySelected >= 4 {
		t.Fatalf("expected directorySelected < 4 after pgup, got %d", updated.directorySelected)
	}
}

func TestUpdateInspectDirectoriesModePgDown(t *testing.T) {
	model := newDirectoriesModel()
	model.directorySelected = 0
	result, _ := updateInspectDirectoriesMode(model, tea.KeyMsg{Type: tea.KeyPgDown})
	updated := result.(inspectModel)
	if updated.directorySelected <= 0 {
		t.Fatalf("expected directorySelected > 0 after pgdown, got %d", updated.directorySelected)
	}
}

func TestUpdateInspectDirectoriesModeEnterOnEmptyNoOp(t *testing.T) {
	model := newDirectoriesModel()
	model.filteredDirectories = nil
	result, _ := updateInspectDirectoriesMode(model, tea.KeyMsg{Type: tea.KeyEnter})
	updated := result.(inspectModel)
	if updated.mode != inspectViewModeDirectories {
		t.Fatal("expected mode unchanged when no directories")
	}
}

// --- inspectTruncateLine ---

func TestInspectTruncateLineZeroWidthReturnsEmpty(t *testing.T) {
	got := inspectTruncateLine("hello", 0)
	if got != "" {
		t.Fatalf("expected empty for width=0, got %q", got)
	}
}

func TestInspectTruncateLineWidthThreeOrLessHardTruncate(t *testing.T) {
	got := inspectTruncateLine("hello world", 3)
	if got != "hel" {
		t.Fatalf("expected 'hel' for width=3, got %q", got)
	}
}

func TestInspectTruncateLineExactLengthNoTruncation(t *testing.T) {
	got := inspectTruncateLine("hello", 5)
	if got != "hello" {
		t.Fatalf("expected 'hello', got %q", got)
	}
}

func TestInspectTruncateLineLongerThanWidthAddsEllipsis(t *testing.T) {
	got := inspectTruncateLine("hello world", 8)
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("expected '...' suffix for truncated line, got %q", got)
	}
	if len([]rune(got)) != 8 {
		t.Fatalf("expected length 8, got %d (%q)", len([]rune(got)), got)
	}
}

// --- inspectDirectoriesVisibleRange start >= end clamp ---

func TestInspectDirectoriesVisibleRangeStartBeyondEndClampsToLast(t *testing.T) {
	model := inspectModel{
		filteredDirectories: []inspectDirectoryRow{{path: "/a"}, {path: "/b"}, {path: "/c"}},
		directoryStart:      999,
		height:              1, // listHeight = max(1-11, 1) = 1
	}
	start, end := inspectDirectoriesVisibleRange(model)
	if start >= end {
		t.Fatalf("expected start < end, got start=%d end=%d", start, end)
	}
}

// --- inspectDocumentsVisibleRange start >= end clamp ---

func TestInspectDocumentsVisibleRangeStartBeyondEndClampsToLast(t *testing.T) {
	model := inspectModel{
		filteredDocuments: []inspectDocumentRow{{name: "a"}, {name: "b"}},
		documentStart:     999,
		height:            1, // listHeight = max(1-12, 1) = 1
	}
	start, end := inspectDocumentsVisibleRange(model)
	if start >= end {
		t.Fatalf("expected start < end, got start=%d end=%d", start, end)
	}
}

// --- inspectLogsVisibleRange start >= end clamp ---

func TestInspectLogsVisibleRangeStartBeyondEndClampsToLast(t *testing.T) {
	model := inspectModel{
		filteredLogs: []inspectLogRow{{path: "/a"}, {path: "/b"}},
		logStart:     999,
		height:       1,
	}
	start, end := inspectLogsVisibleRange(model)
	if start >= end {
		t.Fatalf("expected start < end, got start=%d end=%d", start, end)
	}
}

// --- Update WindowSizeMsg ---

func TestUpdateHandlesWindowSizeMsg(t *testing.T) {
	index := domain.NewInvertedIndex()
	model := newInspectModel(index)
	result, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	updated := result.(inspectModel)
	if updated.width != 120 || updated.height != 40 {
		t.Fatalf("expected width=120 height=40, got %d %d", updated.width, updated.height)
	}
}

func TestUpdateHandlesWindowSizeMsgInJSONMode(t *testing.T) {
	index := domain.NewInvertedIndex()
	model := newInspectModel(index)
	model.mode = inspectViewModeJSON
	model.jsonLines = []string{"line1", "line2"}
	result, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	updated := result.(inspectModel)
	if updated.width != 100 {
		t.Fatalf("expected width=100, got %d", updated.width)
	}
}

func TestUpdateHandlesWindowSizeMsgInDocumentsMode(t *testing.T) {
	index := domain.NewInvertedIndex()
	model := newInspectModel(index)
	model.mode = inspectViewModeDocuments
	result, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	updated := result.(inspectModel)
	if updated.height != 24 {
		t.Fatalf("expected height=24, got %d", updated.height)
	}
}

// --- Update command mode via colon key ---

func TestUpdateEntersCommandModeOnColon(t *testing.T) {
	index := domain.NewInvertedIndex()
	model := newInspectModel(index)
	result, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(":")})
	updated := result.(inspectModel)
	if updated.commandMode != inspectCommandModeCommand {
		t.Fatalf("expected commandModeCommand, got %v", updated.commandMode)
	}
}

func TestUpdateRealtimeRefreshMsgInLogsMode(t *testing.T) {
	index := domain.NewInvertedIndex()
	model := newInspectModel(index)
	model.mode = inspectViewModeLogs
	result, _ := model.Update(inspectRealtimeRefreshMsg{})
	updated := result.(inspectModel)
	if updated.mode != inspectViewModeLogs {
		t.Fatal("expected logs mode preserved after refresh")
	}
}

// --- updateInspectDocumentsSelection up/down boundary cases ---

func TestUpdateInspectDocumentsSelectionUpAtZeroNoOp(t *testing.T) {
	docs := make([]inspectDocumentRow, 5)
	model := inspectModel{filteredDocuments: docs, documentSelected: 0}
	model = updateInspectDocumentsSelection(model, "up")
	if model.documentSelected != 0 {
		t.Fatalf("expected documentSelected=0 (no-op), got %d", model.documentSelected)
	}
}

func TestUpdateInspectDocumentsSelectionUpDecrementsWhenAboveZero(t *testing.T) {
	docs := make([]inspectDocumentRow, 5)
	model := inspectModel{filteredDocuments: docs, documentSelected: 3}
	model = updateInspectDocumentsSelection(model, "k")
	if model.documentSelected != 2 {
		t.Fatalf("expected documentSelected=2, got %d", model.documentSelected)
	}
}

func TestUpdateInspectDocumentsSelectionDownAtMaxNoOp(t *testing.T) {
	docs := make([]inspectDocumentRow, 5)
	model := inspectModel{filteredDocuments: docs, documentSelected: 4}
	model = updateInspectDocumentsSelection(model, "down")
	if model.documentSelected != 4 {
		t.Fatalf("expected documentSelected=4 (no-op at max), got %d", model.documentSelected)
	}
}

func TestUpdateInspectDocumentsSelectionDownIncrementsWhenBelowMax(t *testing.T) {
	docs := make([]inspectDocumentRow, 5)
	model := inspectModel{filteredDocuments: docs, documentSelected: 1}
	model = updateInspectDocumentsSelection(model, "j")
	if model.documentSelected != 2 {
		t.Fatalf("expected documentSelected=2, got %d", model.documentSelected)
	}
}

// --- mergeInspectDocuments nil stats skipped ---

func TestMergeInspectDocumentsSkipsNilStats(t *testing.T) {
	target := domain.NewInvertedIndex()
	source := domain.NewInvertedIndex()
	// Manually insert nil docStats to trigger the nil skip branch
	source.Documents["nilkey"] = nil
	source.Documents["validkey"] = &domain.DocStats{Name: "valid.go", Path: "valid.go", Length: 5}
	mergeInspectDocuments(target, "/repo", source)
	// Only "validkey" should be merged
	if _, ok := target.Documents["/repo::validkey"]; !ok {
		t.Fatal("expected /repo::validkey to be merged")
	}
	if _, ok := target.Documents["/repo::nilkey"]; ok {
		t.Fatal("expected nil key to be skipped")
	}
}

// --- updateInspectCommandInputMode backspace and unknown command ---

func TestUpdateInspectCommandInputModeBackspace(t *testing.T) {
	model := inspectModel{
		commandMode:  inspectCommandModeCommand,
		commandQuery: "ind",
	}
	result, _ := updateInspectCommandInputMode(model, tea.KeyMsg{Type: tea.KeyBackspace})
	updated := result.(inspectModel)
	if updated.commandQuery != "in" {
		t.Fatalf("expected 'in' after backspace, got %q", updated.commandQuery)
	}
}

func TestUpdateInspectCommandInputModeEnterUnknownCommandSetsError(t *testing.T) {
	model := inspectModel{
		commandMode:  inspectCommandModeCommand,
		commandQuery: "bad",
	}
	result, _ := updateInspectCommandInputMode(model, tea.KeyMsg{Type: tea.KeyEnter})
	updated := result.(inspectModel)
	if updated.commandError == "" {
		t.Fatal("expected commandError for unknown command")
	}
}

func TestUpdateInspectCommandInputModeTabAutocompletes(t *testing.T) {
	model := inspectModel{
		commandMode:  inspectCommandModeCommand,
		commandQuery: ":tl",
	}
	result, _ := updateInspectCommandInputMode(model, tea.KeyMsg{Type: tea.KeyTab})
	updated := result.(inspectModel)
	if updated.commandQuery != "tlog" {
		t.Fatalf("expected 'tlog' after tab, got %q", updated.commandQuery)
	}
}

func TestUpdateInspectCommandInputModeKeyRunesAppendsChars(t *testing.T) {
	model := inspectModel{
		commandMode:  inspectCommandModeCommand,
		commandQuery: "in",
	}
	result, _ := updateInspectCommandInputMode(model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	updated := result.(inspectModel)
	if updated.commandQuery != "ind" {
		t.Fatalf("expected 'ind', got %q", updated.commandQuery)
	}
}

// --- updateInspectDocumentSearchMode delete key ---

func TestUpdateInspectDocumentSearchModeDeleteTrims(t *testing.T) {
	model := newDocSearchModel()
	model.documentSearchQuery = "srv"
	result, _ := updateInspectDocumentSearchMode(model, tea.KeyMsg{Type: tea.KeyDelete})
	updated := result.(inspectModel)
	if updated.documentSearchQuery != "sr" {
		t.Fatalf("expected 'sr' after delete, got %q", updated.documentSearchQuery)
	}
}
