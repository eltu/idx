package cli

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	search "idx/internal/features/search"
	"idx/internal/shared/config"
)

// ---- padRight ----

func TestPadRightPadsShortString(t *testing.T) {
	result := padRight("ab", 5)
	if result != "ab   " {
		t.Fatalf("expected %q, got %q", "ab   ", result)
	}
}

func TestPadRightDoesNotTrimWhenAlreadyWide(t *testing.T) {
	result := padRight("abcde", 3)
	if result != "abcde" {
		t.Fatalf("expected unchanged string, got %q", result)
	}
}

func TestPadRightExactWidthIsUnchanged(t *testing.T) {
	result := padRight("abc", 3)
	if result != "abc" {
		t.Fatalf("expected %q, got %q", "abc", result)
	}
}

// ---- formatConfigFloat ----

func TestFormatConfigFloatRendersDecimal(t *testing.T) {
	got := formatConfigFloat(1.5)
	if got != "1.5" {
		t.Fatalf("expected 1.5, got %q", got)
	}
}

func TestFormatConfigFloatRendersZero(t *testing.T) {
	got := formatConfigFloat(0)
	if got != "0" {
		t.Fatalf("expected 0, got %q", got)
	}
}

// ---- formatIgnorePatterns ----

func TestFormatIgnorePatternsEmptySliceReturnsBrackets(t *testing.T) {
	got := formatIgnorePatterns(nil)
	if got != "[]" {
		t.Fatalf("expected [], got %q", got)
	}
}

func TestFormatIgnorePatternsMultiplePatterns(t *testing.T) {
	got := formatIgnorePatterns([]string{"vendor", "*.tmp"})
	if got != "[vendor, *.tmp]" {
		t.Fatalf("expected [vendor, *.tmp], got %q", got)
	}
}

// ---- configTableColumnWidths ----

func TestConfigTableColumnWidthsFindsLongest(t *testing.T) {
	rows := []configRow{
		{key: "ab", value: "x"},
		{key: "long_key", value: "longer_val"},
	}
	maxKey, maxVal := configTableColumnWidths(rows)
	if maxKey != 8 {
		t.Fatalf("expected maxKey=8, got %d", maxKey)
	}
	if maxVal != 10 {
		t.Fatalf("expected maxVal=10, got %d", maxVal)
	}
}

func TestConfigTableColumnWidthsEmptyRows(t *testing.T) {
	maxKey, maxVal := configTableColumnWidths(nil)
	if maxKey != 0 || maxVal != 0 {
		t.Fatalf("expected 0,0 for empty rows, got %d,%d", maxKey, maxVal)
	}
}

// ---- canExecuteWithCobra ----

func TestCanExecuteWithCobraKnownCommands(t *testing.T) {
	known := []string{"sync", "init", "status", "inspect", "read", "watch", "destroy", "search", "version", "skills", "config", "server", "help", "--help", "-h", "--version", "-v"}
	for _, cmd := range known {
		if !canExecuteWithCobra(cmd) {
			t.Errorf("expected %q to be executable with cobra", cmd)
		}
	}
}

func TestCanExecuteWithCobraUnknownCommand(t *testing.T) {
	if canExecuteWithCobra("unknown") {
		t.Fatal("expected unknown command to return false")
	}
}

func TestCanExecuteWithCobraEmptyString(t *testing.T) {
	if canExecuteWithCobra("") {
		t.Fatal("expected empty string to return false")
	}
}

// ---- parseInspectArguments ----

func TestParseInspectArgumentsNoArgsReturnsEmpty(t *testing.T) {
	path, err := parseInspectArguments(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "" {
		t.Fatalf("expected empty path, got %q", path)
	}
}

func TestParseInspectArgumentsSingleArgReturnsPath(t *testing.T) {
	path, err := parseInspectArguments([]string{"/some/dir"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "/some/dir" {
		t.Fatalf("expected /some/dir, got %q", path)
	}
}

func TestParseInspectArgumentsTooManyArgsReturnsError(t *testing.T) {
	_, err := parseInspectArguments([]string{"a", "b"})
	if err == nil {
		t.Fatal("expected error for multiple arguments, got nil")
	}
}

func TestParseInspectArgumentsFlagLikeArgReturnsError(t *testing.T) {
	_, err := parseInspectArguments([]string{"--foo"})
	if err == nil {
		t.Fatal("expected error for flag-like argument, got nil")
	}
}

// ---- LineWriter ----

func TestLineWriterWriteLineWritesToTarget(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewLineWriter(buf)
	if err := w.WriteLine("hello"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "hello") {
		t.Fatalf("expected output to contain 'hello', got %q", buf.String())
	}
}

func TestLineWriterSetQuietSuppressesOutput(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewLineWriter(buf)
	w.SetQuiet(true)
	_ = w.WriteLine("suppressed")
	if buf.Len() != 0 {
		t.Fatalf("expected no output when quiet, got %q", buf.String())
	}
}

func TestLineWriterWriteInlineWritesWithoutNewline(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewLineWriter(buf)
	_ = w.WriteInline("partial")
	if !strings.Contains(buf.String(), "partial") {
		t.Fatalf("expected output to contain 'partial', got %q", buf.String())
	}
	if strings.HasSuffix(buf.String(), "\n") {
		t.Fatal("expected no trailing newline from WriteInline")
	}
}

func TestLineWriterWriteInlineQuietSuppresses(t *testing.T) {
	buf := &bytes.Buffer{}
	w := NewLineWriter(buf)
	w.SetQuiet(true)
	_ = w.WriteInline("suppressed")
	if buf.Len() != 0 {
		t.Fatalf("expected no output when quiet, got %q", buf.String())
	}
}

// ---- NewCommandRunner + With* builders ----

func TestNewCommandRunnerHasDefaultConfig(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	def := config.DefaultIdxConfig()
	if runner.config.Search.Format != def.Search.Format {
		t.Fatalf("expected default search format %q, got %q", def.Search.Format, runner.config.Search.Format)
	}
}

func TestWithBuildInfoAttachesVersionAndDate(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).
		WithBuildInfo(BuildInfo{Version: "v1.2.3", BuildDate: "2026-01-01T00:00:00Z"})
	if runner.buildInfo.Version != "v1.2.3" {
		t.Fatalf("expected version v1.2.3, got %q", runner.buildInfo.Version)
	}
}

func TestWithConfigAttachesConfigAndOverrides(t *testing.T) {
	cfg := config.DefaultIdxConfig()
	cfg.Search.Format = search.OutputJSON
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).
		WithConfig(cfg, "/project/.idx.yml", []string{"search.format"})
	if runner.configFilePath != "/project/.idx.yml" {
		t.Fatalf("expected config path, got %q", runner.configFilePath)
	}
	if len(runner.configOverrides) != 1 || runner.configOverrides[0] != "search.format" {
		t.Fatalf("expected overrides [search.format], got %v", runner.configOverrides)
	}
}

func TestWithQuietToggleWiresTarget(t *testing.T) {
	buf := &bytes.Buffer{}
	writer := NewLineWriter(buf)
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).
		WithQuietToggle(writer)
	if runner.quietToggle == nil {
		t.Fatal("expected quietToggle to be set")
	}
}

// ---- validateSearchFlagValues ----

func TestValidateSearchFlagValuesNegativeContextErrors(t *testing.T) {
	err := validateSearchFlagValues(-1, 0, 0, false)
	if err == nil {
		t.Fatal("expected error for negative context")
	}
}

func TestValidateSearchFlagValuesNegativeFromErrors(t *testing.T) {
	err := validateSearchFlagValues(0, -1, 0, false)
	if err == nil {
		t.Fatal("expected error for negative from")
	}
}

func TestValidateSearchFlagValuesNegativeSizeErrors(t *testing.T) {
	err := validateSearchFlagValues(0, 0, -1, false)
	if err == nil {
		t.Fatal("expected error for negative size")
	}
}

func TestValidateSearchFlagValuesZeroSizeWithChangedFlagErrors(t *testing.T) {
	err := validateSearchFlagValues(0, 0, 0, true)
	if err == nil {
		t.Fatal("expected error for size=0 when sizeChanged=true")
	}
}

func TestValidateSearchFlagValuesValidInputNoError(t *testing.T) {
	if err := validateSearchFlagValues(2, 0, 10, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---- validateSearchFormat ----

func TestValidateSearchFormatRejectsUnknown(t *testing.T) {
	if err := validateSearchFormat("xml", false); err == nil {
		t.Fatal("expected error for unknown format")
	}
}

func TestValidateSearchFormatRejectsPrettyJSONWithTextFormat(t *testing.T) {
	if err := validateSearchFormat(search.OutputText, true); err == nil {
		t.Fatal("expected error when prettyJSON=true with text format")
	}
}

func TestValidateSearchFormatAcceptsJSON(t *testing.T) {
	if err := validateSearchFormat(search.OutputJSON, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---- validateSearchOperator ----

func TestValidateSearchOperatorRejectsUnknown(t *testing.T) {
	if err := validateSearchOperator("XOR"); err == nil {
		t.Fatal("expected error for unknown operator")
	}
}

func TestValidateSearchOperatorAcceptsAND(t *testing.T) {
	if err := validateSearchOperator(search.OperatorAND); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateSearchOperatorAcceptsOR(t *testing.T) {
	if err := validateSearchOperator(search.OperatorOR); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---- validateSearchRelaxation ----

func TestValidateSearchRelaxationEmptyStringIsValid(t *testing.T) {
	cfg := &searchCommandConfig{operator: search.OperatorAND}
	if err := validateSearchRelaxation(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateSearchRelaxationRequiresANDOperator(t *testing.T) {
	cfg := &searchCommandConfig{operator: search.OperatorOR, relaxation: ">2"}
	if err := validateSearchRelaxation(cfg); err == nil {
		t.Fatal("expected error when relaxation used with OR operator")
	}
}

func TestValidateSearchRelaxationValidFormat(t *testing.T) {
	cfg := &searchCommandConfig{operator: search.OperatorAND, relaxation: ">2"}
	if err := validateSearchRelaxation(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.relaxationEnabled || cfg.relaxationMin != 2 {
		t.Fatalf("expected relaxationEnabled=true relaxationMin=2, got %v/%d", cfg.relaxationEnabled, cfg.relaxationMin)
	}
}

func TestValidateSearchRelaxationInvalidFormat(t *testing.T) {
	cfg := &searchCommandConfig{operator: search.OperatorAND, relaxation: "2"}
	if err := validateSearchRelaxation(cfg); err == nil {
		t.Fatal("expected error for missing > prefix")
	}
}

// ---- formatVersionDate ----

func TestFormatVersionDateValidRFC3339(t *testing.T) {
	date := "2026-01-15T12:00:00Z"
	result := formatVersionDate(date)
	if result == date {
		t.Fatal("expected formatted date, got raw RFC3339 string")
	}
	if !strings.Contains(result, "2026") {
		t.Fatalf("expected year 2026 in result, got %q", result)
	}
}

func TestFormatVersionDateInvalidPassthrough(t *testing.T) {
	raw := "not-a-date"
	result := formatVersionDate(raw)
	if result != raw {
		t.Fatalf("expected passthrough for invalid date, got %q", result)
	}
}

// ---- defaultConfigValues ----

func TestDefaultConfigValuesHasExpectedKeys(t *testing.T) {
	def := defaultConfigValues()
	required := []string{"search.format", "search.size", "bm25.k1", "bm25.b", "log.level"}
	for _, key := range required {
		if _, ok := def[key]; !ok {
			t.Errorf("expected key %q in default config values", key)
		}
	}
}

// ---- buildConfigRows ----

func TestBuildConfigRowsReturnsAllKeys(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	rows := buildConfigRows(runner)
	if len(rows) == 0 {
		t.Fatal("expected non-empty config rows")
	}
	found := false
	for _, r := range rows {
		if r.key == "search.format" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected search.format in config rows")
	}
}

// ---- validateSearchInput ----

func TestValidateSearchInputNoQueryAndNoFiltersErrors(t *testing.T) {
	err := validateSearchInput("", nil, nil, []string{"idx", "search"})
	if err == nil {
		t.Fatal("expected error for empty query with no filters")
	}
}

func TestValidateSearchInputWithQueryNoError(t *testing.T) {
	if err := validateSearchInput("foo", nil, nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateSearchInputExtFilterAloneIsValid(t *testing.T) {
	if err := validateSearchInput("", nil, []string{"go"}, nil); err != nil {
		t.Fatalf("unexpected error with ext filter: %v", err)
	}
}

// ---- currentDirTilde ----

func TestCurrentDirTildeReturnsNonEmpty(t *testing.T) {
	result := currentDirTilde()
	if result == "" {
		t.Fatal("expected non-empty path from currentDirTilde")
	}
}

// ---- searchCommandConfig.options ----

func TestSearchCommandConfigOptionsMapsFormat(t *testing.T) {
	cfg := searchCommandConfig{
		format:   search.OutputJSON,
		operator: search.OperatorAND,
	}
	opts := cfg.options()
	if opts.Format != search.OutputJSON {
		t.Fatalf("expected JSON format, got %q", opts.Format)
	}
}

func TestSearchCommandConfigOptionsPathQuerySetWhenPresent(t *testing.T) {
	cfg := searchCommandConfig{format: search.OutputText, operator: search.OperatorAND, pathQueries: []string{"internal/core"}}
	opts := cfg.options()
	if opts.PathQuery != "internal/core" {
		t.Fatalf("expected PathQuery=internal/core, got %q", opts.PathQuery)
	}
}

func TestSearchCommandConfigOptionsExtensionQuerySetWhenPresent(t *testing.T) {
	cfg := searchCommandConfig{format: search.OutputText, operator: search.OperatorAND, extensionQueries: []string{"go"}}
	opts := cfg.options()
	if opts.ExtensionQuery != "go" {
		t.Fatalf("expected ExtensionQuery=go, got %q", opts.ExtensionQuery)
	}
}

func TestSearchCommandConfigOptionsMapsOperator(t *testing.T) {
	cfg := searchCommandConfig{format: search.OutputText, operator: search.OperatorOR}
	opts := cfg.options()
	if opts.Operator != search.OperatorOR {
		t.Fatalf("expected OR operator, got %q", opts.Operator)
	}
}

func TestSearchCommandConfigOptionsMatchesOnlyFlagSet(t *testing.T) {
	cfg := searchCommandConfig{format: search.OutputText, operator: search.OperatorAND, matchesOnly: true}
	opts := cfg.options()
	if !opts.MatchesOnly {
		t.Fatal("expected MatchesOnly=true")
	}
}

func TestSearchCommandConfigOptionsLegacyMatchesOnlySetsFlag(t *testing.T) {
	cfg := searchCommandConfig{format: search.OutputText, operator: search.OperatorAND, legacyMatchesOnly: true}
	opts := cfg.options()
	if !opts.MatchesOnly {
		t.Fatal("expected MatchesOnly=true via legacyMatchesOnly")
	}
}

// ---- showConfigBanner / writeConfigDetails / printConfigNoFileMessage / printConfigTable ----

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	return buf.String()
}

func TestShowConfigBannerSilentWithNoFile(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	// configFilePath is empty → showConfigBanner should print nothing
	output := captureStdout(t, func() { runner.showConfigBanner() })
	if output != "" {
		t.Fatalf("expected no output when configFilePath is empty, got %q", output)
	}
}

func TestShowConfigBannerWithFilePrintsPath(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).
		WithConfig(config.DefaultIdxConfig(), "/project/.idx.yml", nil)
	output := captureStdout(t, func() { runner.showConfigBanner() })
	if !strings.Contains(output, ".idx.yml") {
		t.Fatalf("expected config path in banner, got %q", output)
	}
}

func TestShowConfigBannerWithOverridesMentionsCount(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).
		WithConfig(config.DefaultIdxConfig(), "/project/.idx.yml", []string{"search.format"})
	output := captureStdout(t, func() { runner.showConfigBanner() })
	if !strings.Contains(output, "1") {
		t.Fatalf("expected override count in banner output, got %q", output)
	}
}

func TestWriteConfigDetailsNoFilePathPrintsNoFileMessage(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	output := captureStdout(t, func() {
		if err := runner.writeConfigDetails(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(output, "No .idx.yml") {
		t.Fatalf("expected no-file message, got %q", output)
	}
}

func TestWriteConfigDetailsWithFilePathPrintsTable(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).
		WithConfig(config.DefaultIdxConfig(), "/project/.idx.yml", []string{"search.format"})
	output := captureStdout(t, func() {
		if err := runner.writeConfigDetails(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(output, "search.format") {
		t.Fatalf("expected config table with search.format, got %q", output)
	}
}

func TestPrintConfigNoFileMessageContainsTip(t *testing.T) {
	output := captureStdout(t, printConfigNoFileMessage)
	if !strings.Contains(output, "Tip:") && !strings.Contains(output, "tip") && !strings.Contains(output, ".idx.yml") {
		t.Fatalf("expected tip in no-file message, got %q", output)
	}
}

func TestPrintConfigTableWithOverrideShowsSource(t *testing.T) {
	rows := []configRow{
		{key: "search.format", value: "json", defaultValue: "text"},
	}
	overrideSet := map[string]bool{"search.format": true}
	output := captureStdout(t, func() { printConfigTable(rows, overrideSet) })
	if !strings.Contains(output, "search.format") {
		t.Fatalf("expected key in table output, got %q", output)
	}
}

func TestPrintConfigTableWithoutOverrideShowsDefault(t *testing.T) {
	rows := []configRow{
		{key: "search.format", value: "text", defaultValue: "text"},
	}
	output := captureStdout(t, func() { printConfigTable(rows, map[string]bool{}) })
	if !strings.Contains(output, "search.format") {
		t.Fatalf("expected key in table output, got %q", output)
	}
}

// ---- isIgnorableServerStopError ----

func TestIsIgnorableServerStopErrorNotRunning(t *testing.T) {
	err := errors.New("server not running")
	if !isIgnorableServerStopError(err) {
		t.Fatal("expected not-running to be ignorable")
	}
}

func TestIsIgnorableServerStopErrorStateNotFound(t *testing.T) {
	err := errors.New("state not found")
	if !isIgnorableServerStopError(err) {
		t.Fatal("expected state-not-found to be ignorable")
	}
}

func TestIsIgnorableServerStopErrorNotFound(t *testing.T) {
	err := errors.New("path not found")
	if !isIgnorableServerStopError(err) {
		t.Fatal("expected not-found to be ignorable")
	}
}

func TestIsIgnorableServerStopErrorOtherErrorNotIgnorable(t *testing.T) {
	err := errors.New("permission denied")
	if isIgnorableServerStopError(err) {
		t.Fatal("expected permission denied to not be ignorable")
	}
}

// ---- WithSkillsCommand / WithReadCommand ----

type stubSkillsCommand struct{}

func (s stubSkillsCommand) Install(editor string) error { return nil }

type stubReadCommand struct{}

func (s stubReadCommand) RunWithOptions(filePath string, fromLine, toLine int) error { return nil }

func TestWithSkillsCommandSetsField(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).
		WithSkillsCommand(stubSkillsCommand{})
	if runner.skillsCommand == nil {
		t.Fatal("expected skillsCommand to be set")
	}
}

func TestWithReadCommandSetsField(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).
		WithReadCommand(stubReadCommand{})
	if runner.readCommand == nil {
		t.Fatal("expected readCommand to be set")
	}
}

// ---- InitCommandAdapter ----

type stubInitRunner struct{ runErr error }

func (s stubInitRunner) Run() error { return s.runErr }

func TestInitCommandAdapterRunFromPathChangesToProjectDir(t *testing.T) {
	dir := t.TempDir()
	adapter := NewInitCommandAdapter(stubInitRunner{}, nil)
	if err := adapter.RunFromPath(dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInitCommandAdapterRunFromPathInvalidDirReturnsError(t *testing.T) {
	adapter := NewInitCommandAdapter(stubInitRunner{}, nil)
	err := adapter.RunFromPath("/nonexistent/path/xyz")
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
}

func TestInitCommandAdapterRunFromPathPropagatesRunError(t *testing.T) {
	dir := t.TempDir()
	adapter := NewInitCommandAdapter(stubInitRunner{runErr: errors.New("index failed")}, nil)
	err := adapter.RunFromPath(dir)
	if err == nil {
		t.Fatal("expected error from Run()")
	}
	if !strings.Contains(err.Error(), "index failed") {
		t.Fatalf("expected run error in message, got %q", err.Error())
	}
}

// ---- renderVersionOutput ----

func TestRenderVersionOutputContainsVersion(t *testing.T) {
	output := renderVersionOutput("v9.9.9", "2026-01-01T00:00:00Z")
	if !strings.Contains(output, "v9.9.9") {
		t.Fatalf("expected version in output, got %q", output)
	}
}

func TestRenderVersionOutputContainsBuildDate(t *testing.T) {
	output := renderVersionOutput("v1.0.0", "2026-06-15T10:00:00Z")
	if !strings.Contains(output, "2026") {
		t.Fatalf("expected year in output, got %q", output)
	}
}
