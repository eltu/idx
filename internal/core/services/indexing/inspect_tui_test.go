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

func TestInspectCommonPrefixReturnsSharedPrefix(t *testing.T) {
	tests := []struct {
		left     string
		right    string
		expected string
	}{
		{"index", "indexer", "index"},
		{"tlog", "tlog", "tlog"},
		{"abc", "xyz", ""},
		{"index", "tlog", ""},
		{"", "tlog", ""},
		{"tlog", "", ""},
	}

	for _, tc := range tests {
		got := inspectCommonPrefix(tc.left, tc.right)
		if got != tc.expected {
			t.Fatalf("inspectCommonPrefix(%q, %q) = %q, want %q", tc.left, tc.right, got, tc.expected)
		}
	}
}

func TestInspectCommandSuggestionsReturnsAllOnEmpty(t *testing.T) {
	suggestions := inspectCommandSuggestions("")
	if len(suggestions) != len(inspectAvailableCommands) {
		t.Fatalf("expected %d suggestions for empty query, got %d", len(inspectAvailableCommands), len(suggestions))
	}
}

func TestInspectCommandSuggestionsFiltersPrefix(t *testing.T) {
	suggestions := inspectCommandSuggestions("in")
	if len(suggestions) != 1 || suggestions[0] != "index" {
		t.Fatalf("expected [index] for prefix 'in', got %v", suggestions)
	}
}

func TestInspectCommandSuggestionsReturnsEmptyForNoMatch(t *testing.T) {
	suggestions := inspectCommandSuggestions("xyz")
	if len(suggestions) != 0 {
		t.Fatalf("expected empty suggestions for unknown prefix 'xyz', got %v", suggestions)
	}
}

func TestAutocompleteInspectCommandSingleMatchCompletes(t *testing.T) {
	result := autocompleteInspectCommand(":tl")
	if result != "tlog" {
		t.Fatalf("expected 'tlog', got %q", result)
	}
}

func TestAutocompleteInspectCommandNoMatchReturnsQuery(t *testing.T) {
	result := autocompleteInspectCommand(":xyz")
	if result != ":xyz" {
		t.Fatalf("expected original query ':xyz', got %q", result)
	}
}

func TestAutocompleteInspectCommandEmptyQueryReturnsQuery(t *testing.T) {
	result := autocompleteInspectCommand(":")
	if result != ":" {
		t.Fatalf("expected original query ':', got %q", result)
	}
}

func TestAutocompleteInspectCommandMultipleMatchesExpandsCommonPrefix(t *testing.T) {
	// Both "index" and any future "i*" command would share prefix;
	// with current commands ":i" only matches "index" so returns "index".
	result := autocompleteInspectCommand(":i")
	if result != "index" {
		t.Fatalf("expected 'index' for ':i', got %q", result)
	}
}

func TestInspectStringFieldReturnsFirstNonEmpty(t *testing.T) {
	fields := map[string]any{
		"a": "  ",
		"b": "hello",
		"c": "world",
	}

	got := inspectStringField(fields, "a", "b", "c")
	if got != "hello" {
		t.Fatalf("expected 'hello', got %q", got)
	}
}

func TestInspectStringFieldSkipsMissingKeys(t *testing.T) {
	fields := map[string]any{"b": "found"}
	got := inspectStringField(fields, "missing", "b")
	if got != "found" {
		t.Fatalf("expected 'found', got %q", got)
	}
}

func TestInspectStringFieldSkipsNonStringValues(t *testing.T) {
	fields := map[string]any{"a": 42, "b": "ok"}
	got := inspectStringField(fields, "a", "b")
	if got != "ok" {
		t.Fatalf("expected 'ok', got %q", got)
	}
}

func TestInspectStringFieldReturnsEmptyWhenAllEmpty(t *testing.T) {
	fields := map[string]any{"a": "  "}
	got := inspectStringField(fields, "a", "missing")
	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestExtractSummaryValueEqualsSign(t *testing.T) {
	summary := "indexed_at=2026-05-01T12:00:00Z path=/repo hash=abc"
	got := extractSummaryValue(summary, "path")
	if got != "/repo" {
		t.Fatalf("expected '/repo', got %q", got)
	}
}

func TestExtractSummaryValueColonSeparator(t *testing.T) {
	summary := "path:/repo/internal hash:abc123"
	got := extractSummaryValue(summary, "hash")
	if got != "abc123" {
		t.Fatalf("expected 'abc123', got %q", got)
	}
}

func TestExtractSummaryValueMissingKeyReturnsEmpty(t *testing.T) {
	got := extractSummaryValue("path=/repo", "missing")
	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestExtractSummaryValueReturnsUntilDelimiter(t *testing.T) {
	summary := "indexed_at=2026-05-01,path=/repo"
	got := extractSummaryValue(summary, "indexed_at")
	if got != "2026-05-01" {
		t.Fatalf("expected '2026-05-01', got %q", got)
	}
}

func TestParseInspectSummaryFieldsEmptyReturnsEmpty(t *testing.T) {
	a, b, c := parseInspectSummaryFields("   ")
	if a != "" || b != "" || c != "" {
		t.Fatalf("expected all empty for whitespace input, got %q %q %q", a, b, c)
	}
}

func TestParseInspectSummaryFieldsColonSeparator(t *testing.T) {
	indexedAt, pathValue, hash := parseInspectSummaryFields("indexed_at:2026-05-01T00:00:00Z path:/repo hash:deadbeef")
	if pathValue != "/repo" {
		t.Fatalf("expected '/repo', got %q", pathValue)
	}
	if hash != "deadbeef" {
		t.Fatalf("expected 'deadbeef', got %q", hash)
	}
	_ = indexedAt
}

func TestTrimLastRuneRemovesLastCharacter(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hell"},
		{"a", ""},
		{"", ""},
		{"café", "caf"},
	}

	for _, tc := range tests {
		got := trimLastRune(tc.input)
		if got != tc.expected {
			t.Fatalf("trimLastRune(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestInspectBackspaceInDirectorySearchTrimsCursor(t *testing.T) {
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

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	updatedModel := updated.(inspectModel)
	if updatedModel.directorySearchQuery != "doc" {
		t.Fatalf("expected 'doc' after backspace, got %q", updatedModel.directorySearchQuery)
	}
}

func TestInspectBackspaceInLogSearchTrimsCursor(t *testing.T) {
	model := inspectModel{
		mode:           inspectViewModeLogs,
		logSearchMode:  true,
		logSearchQuery: "repo",
		logs:           []inspectLogRow{{path: "/repo", indexedAt: "2026-05-01T00:00:00Z", hash: "abc"}},
		filteredLogs:   []inspectLogRow{{path: "/repo", indexedAt: "2026-05-01T00:00:00Z", hash: "abc"}},
	}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	updatedModel := updated.(inspectModel)
	if updatedModel.logSearchQuery != "rep" {
		t.Fatalf("expected 'rep' after backspace, got %q", updatedModel.logSearchQuery)
	}
}

func TestInspectDocumentsVisibleRangeEmpty(t *testing.T) {
	model := inspectModel{}
	start, end := inspectDocumentsVisibleRange(model)
	if start != 0 || end != 0 {
		t.Fatalf("expected (0,0) for empty documents, got (%d, %d)", start, end)
	}
}

func TestInspectDocumentsVisibleRangeUsesHeight(t *testing.T) {
	model := inspectModel{
		filteredDocuments: make([]inspectDocumentRow, 20),
		documentStart:     2,
		height:            16,
	}

	start, end := inspectDocumentsVisibleRange(model)
	if start != 2 {
		t.Fatalf("expected start=2, got %d", start)
	}
	if end <= start {
		t.Fatalf("expected end > start, got end=%d", end)
	}
}

func TestInspectLogsVisibleRangeEmpty(t *testing.T) {
	model := inspectModel{}
	start, end := inspectLogsVisibleRange(model)
	if start != 0 || end != 0 {
		t.Fatalf("expected (0,0) for empty logs, got (%d, %d)", start, end)
	}
}

func TestInspectLogsVisibleRangeUsesHeight(t *testing.T) {
	model := inspectModel{
		filteredLogs: make([]inspectLogRow, 20),
		logStart:     1,
		height:       16,
	}

	start, end := inspectLogsVisibleRange(model)
	if start != 1 {
		t.Fatalf("expected start=1, got %d", start)
	}
	if end <= start {
		t.Fatalf("expected end > start, got end=%d", end)
	}
}

func TestInspectLogsPageStepMinimumOne(t *testing.T) {
	model := inspectModel{height: 0}
	step := inspectLogsPageStep(model)
	if step < 1 {
		t.Fatalf("expected page step >= 1, got %d", step)
	}
}

func TestInspectLogsPageStepWithNormalHeight(t *testing.T) {
	model := inspectModel{height: 20}
	step := inspectLogsPageStep(model)
	if step < 1 {
		t.Fatalf("expected positive page step, got %d", step)
	}
}

func TestInspectJSONRangeEmpty(t *testing.T) {
	model := inspectModel{}
	start, end := inspectJSONRange(model)
	if start != 0 || end != 0 {
		t.Fatalf("expected (0,0) for empty JSON lines, got (%d, %d)", start, end)
	}
}

func TestInspectJSONRangeUsesHeight(t *testing.T) {
	model := inspectModel{
		jsonLines: make([]string, 30),
		jsonStart: 3,
		height:    20,
	}

	start, end := inspectJSONRange(model)
	if start != 3 {
		t.Fatalf("expected start=3, got %d", start)
	}
	if end <= start {
		t.Fatalf("expected end > start, got end=%d", end)
	}
}

func TestInspectDividerWidthClampsMinimum(t *testing.T) {
	if inspectDividerWidth(4) != 8 {
		t.Fatal("expected minimum divider width of 8")
	}
}

func TestInspectDividerWidthClampsMaximum(t *testing.T) {
	if inspectDividerWidth(200) != 120 {
		t.Fatal("expected maximum divider width of 120")
	}
}

func TestInspectDividerWidthNormalValue(t *testing.T) {
	if inspectDividerWidth(80) != 80 {
		t.Fatal("expected divider width of 80 to be returned as-is")
	}
}

func TestInspectBuildLogRowPlainText(t *testing.T) {
	row := inspectBuildLogRow("plain summary text", 0, "/repo/.idx/logs/tlog.idx")
	if row.summary != "plain summary text" {
		t.Fatalf("expected plain text as summary, got %q", row.summary)
	}
	if row.jsonRaw != "plain summary text" {
		t.Fatalf("expected jsonRaw to equal raw line, got %q", row.jsonRaw)
	}
}

func TestInspectBuildLogRowJSONLine(t *testing.T) {
	line := `{"indexed_at":"2026-05-01T12:00:00Z","path":"/repo/internal","hash":"abc123","summary":"ok"}`
	row := inspectBuildLogRow(line, 0, "/repo/.idx/logs/tlog.idx")
	if row.indexedAt != "2026-05-01T12:00:00Z" {
		t.Fatalf("expected indexed_at from JSON, got %q", row.indexedAt)
	}
	if row.path != "/repo/internal" {
		t.Fatalf("expected path from JSON, got %q", row.path)
	}
	if row.hash != "abc123" {
		t.Fatalf("expected hash from JSON, got %q", row.hash)
	}
	if row.summary != "ok" {
		t.Fatalf("expected summary from JSON, got %q", row.summary)
	}
}

func TestInspectBuildLogRowMissingFieldsFallbackToDash(t *testing.T) {
	row := inspectBuildLogRow(`{"other":"value"}`, 0, "/repo/.idx/logs/tlog.idx")
	if row.indexedAt != "-" {
		t.Fatalf("expected '-' for missing indexed_at, got %q", row.indexedAt)
	}
	if row.hash != "-" {
		t.Fatalf("expected '-' for missing hash, got %q", row.hash)
	}
}

func TestInspectDocumentDirectoryFromKeyWithSeparator(t *testing.T) {
	got := inspectDocumentDirectory("/repo/internal::main.go", "/repo/internal/main.go")
	if got != "/repo/internal" {
		t.Fatalf("expected '/repo/internal' from key separator, got %q", got)
	}
}

func TestInspectDocumentDirectoryFromPathFallback(t *testing.T) {
	got := inspectDocumentDirectory("doc1", "/repo/internal/core/service.go")
	if got != "/repo/internal/core" {
		t.Fatalf("expected '/repo/internal/core' from path, got %q", got)
	}
}

func TestInspectDocumentDirectoryEmptyPathReturnsDot(t *testing.T) {
	got := inspectDocumentDirectory("doc1", "")
	if got != "." {
		t.Fatalf("expected '.' for empty path, got %q", got)
	}
}

func TestInspectDocumentDirectoryRootFilenameReturnsFilename(t *testing.T) {
	got := inspectDocumentDirectory("doc1", "main.go")
	if got != "main.go" {
		t.Fatalf("expected 'main.go' for root-level file with no slash, got %q", got)
	}
}

func TestInspectRowsFromIndexNilReturnsEmpty(t *testing.T) {
	dirs, byDir := inspectRowsFromIndex(nil)
	if len(dirs) != 0 || len(byDir) != 0 {
		t.Fatalf("expected empty results for nil index, got dirs=%d byDir=%d", len(dirs), len(byDir))
	}
}

func TestInspectRowsFromIndexBuildsDirectoriesAndDocuments(t *testing.T) {
	index := domain.NewInvertedIndex()
	index.AddDocument("/repo/internal::service.go", "internal/service.go", 10)
	index.AddDocument("/repo/internal::core.go", "internal/core.go", 5)
	index.AddDocument("/repo/cmd::main.go", "cmd/main.go", 3)

	dirs, byDir := inspectRowsFromIndex(index)
	if len(dirs) != 2 {
		t.Fatalf("expected 2 directories, got %d", len(dirs))
	}
	if len(byDir["/repo/internal"]) != 2 {
		t.Fatalf("expected 2 documents in /repo/internal, got %d", len(byDir["/repo/internal"]))
	}
	if len(byDir["/repo/cmd"]) != 1 {
		t.Fatalf("expected 1 document in /repo/cmd, got %d", len(byDir["/repo/cmd"]))
	}
}

func TestViewReturnsNonEmptyInDirectoriesMode(t *testing.T) {
	index := domain.NewInvertedIndex()
	index.AddDocument("/repo/internal::service.go", "internal/service.go", 10)
	model := newInspectModel(index)
	model.height = 24
	model.width = 80

	view := model.View()
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

	view := model.View()
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

	view := model.View()
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

	view := model.View()
	if view == "" {
		t.Fatal("expected non-empty view in JSON mode")
	}
}

func TestViewReturnsQuitMessageWhenQuitting(t *testing.T) {
	model := newInspectModel(domain.NewInvertedIndex())
	model.quitting = true

	view := model.View()
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
