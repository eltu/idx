package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"idx/internal/features/indexing"
)

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

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
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

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
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
	index := indexing.NewInvertedIndex()
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
