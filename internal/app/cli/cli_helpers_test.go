package cli

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	search "idx/internal/features/search"
	"idx/internal/shared/config"
)

// padRight, formatConfigFloat, formatIgnorePatterns moved to shared/config.
// Tests for those functions are in internal/shared/config/format_test.go.

// ---- configTableColumnWidths ----

func TestConfigTableColumnWidths_FindsLongestKeyAndValue(t *testing.T) {
	t.Parallel()
	rows := []configRow{
		{key: "ab", value: "x"},
		{key: "long_key", value: "longer_val"},
	}
	maxKey, maxVal := configTableColumnWidths(rows)
	assert.Equal(t, 8, maxKey)
	assert.Equal(t, 10, maxVal)
}

func TestConfigTableColumnWidths_EmptyRows_ReturnsZeros(t *testing.T) {
	t.Parallel()
	maxKey, maxVal := configTableColumnWidths(nil)
	assert.Equal(t, 0, maxKey)
	assert.Equal(t, 0, maxVal)
}

// ---- canExecuteWithCobra ----

func TestCanExecuteWithCobra_KnownCommands_ReturnTrue(t *testing.T) {
	t.Parallel()
	known := []string{"sync", "init", "status", "inspect", "read", "watch", "destroy", "search", "version", "skills", "config", "server", "help", "--help", "-h", "--version", "-v"}
	for _, cmd := range known {
		cmd := cmd
		t.Run(cmd, func(t *testing.T) {
			t.Parallel()
			assert.True(t, canExecuteWithCobra(cmd), "expected %q to be executable with cobra", cmd)
		})
	}
}

func TestCanExecuteWithCobra_UnknownCommand_ReturnsFalse(t *testing.T) {
	t.Parallel()
	assert.False(t, canExecuteWithCobra("unknown"))
}

func TestCanExecuteWithCobra_EmptyString_ReturnsFalse(t *testing.T) {
	t.Parallel()
	assert.False(t, canExecuteWithCobra(""))
}

// ---- parseInspectArguments ----

func TestParseInspectArguments_NoArgs_ReturnsEmpty(t *testing.T) {
	t.Parallel()
	path, err := parseInspectArguments(nil)
	require.NoError(t, err)
	assert.Empty(t, path)
}

func TestParseInspectArguments_SingleArg_ReturnsPath(t *testing.T) {
	t.Parallel()
	path, err := parseInspectArguments([]string{"/some/dir"})
	require.NoError(t, err)
	assert.Equal(t, "/some/dir", path)
}

func TestParseInspectArguments_TooManyArgs_ReturnsError(t *testing.T) {
	t.Parallel()
	_, err := parseInspectArguments([]string{"a", "b"})
	require.Error(t, err)
}

func TestParseInspectArguments_FlagLikeArg_ReturnsError(t *testing.T) {
	t.Parallel()
	_, err := parseInspectArguments([]string{"--foo"})
	require.Error(t, err)
}

// ---- LineWriter ----

func TestLineWriter_WriteLine_WritesToTarget(t *testing.T) {
	t.Parallel()

	// Arrange
	buf := &bytes.Buffer{}
	w := NewLineWriter(buf)

	// Act
	err := w.WriteLine("hello")

	// Assert
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "hello")
}

func TestLineWriter_SetQuiet_SuppressesOutput(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	w := NewLineWriter(buf)
	w.SetQuiet(true)
	_ = w.WriteLine("suppressed")
	assert.Empty(t, buf.String())
}

func TestLineWriter_WriteInline_WritesWithoutNewline(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	w := NewLineWriter(buf)
	_ = w.WriteInline("partial")
	assert.Contains(t, buf.String(), "partial")
	assert.False(t, strings.HasSuffix(buf.String(), "\n"), "expected no trailing newline from WriteInline")
}

func TestLineWriter_WriteInline_QuietSuppresses(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	w := NewLineWriter(buf)
	w.SetQuiet(true)
	_ = w.WriteInline("suppressed")
	assert.Empty(t, buf.String())
}

// ---- NewCommandRunner + With* builders ----

func TestNewCommandRunner_HasDefaultConfig(t *testing.T) {
	t.Parallel()
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	def := config.DefaultIdxConfig()
	assert.Equal(t, def.Search.Format, runner.config.Search.Format)
}

func TestWithBuildInfo_AttachesVersionAndDate(t *testing.T) {
	t.Parallel()
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).
		WithBuildInfo(BuildInfo{Version: "v1.2.3", BuildDate: "2026-01-01T00:00:00Z"})
	assert.Equal(t, "v1.2.3", runner.buildInfo.Version)
}

func TestWithConfig_AttachesConfigAndOverrides(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultIdxConfig()
	cfg.Search.Format = search.OutputJSON
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).
		WithConfig(cfg, "/project/.idx.yml", []string{"search.format"})
	assert.Equal(t, "/project/.idx.yml", runner.configFilePath)
	assert.Equal(t, []string{"search.format"}, runner.configOverrides)
}

func TestWithQuietToggle_WiresTarget(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	writer := NewLineWriter(buf)
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).
		WithQuietToggle(writer)
	assert.NotNil(t, runner.quietToggle)
}

// ---- validateSearchFlagValues ----

func TestValidateSearchFlagValues_NegativeOrZeroInvalid_ReturnsError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		context     int
		from        int
		size        int
		sizeChanged bool
	}{
		{"negative context", -1, 0, 0, false},
		{"negative from", 0, -1, 0, false},
		{"negative size", 0, 0, -1, false},
		{"zero size with sizeChanged", 0, 0, 0, true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateSearchFlagValues(tc.context, tc.from, tc.size, tc.sizeChanged)
			require.Error(t, err)
		})
	}
}

func TestValidateSearchFlagValues_ValidInput_NoError(t *testing.T) {
	t.Parallel()
	assert.NoError(t, validateSearchFlagValues(2, 0, 10, true))
}

// ---- validateSearchFormat ----

func TestValidateSearchFormat_RejectsUnknown(t *testing.T) {
	t.Parallel()
	require.Error(t, validateSearchFormat("xml", false))
}

func TestValidateSearchFormat_RejectsPrettyJSONWithTextFormat(t *testing.T) {
	t.Parallel()
	require.Error(t, validateSearchFormat(search.OutputText, true))
}

func TestValidateSearchFormat_AcceptsJSON(t *testing.T) {
	t.Parallel()
	assert.NoError(t, validateSearchFormat(search.OutputJSON, false))
}

// ---- validateSearchOperator ----

func TestValidateSearchOperator_RejectsUnknown(t *testing.T) {
	t.Parallel()
	require.Error(t, validateSearchOperator("XOR"))
}

func TestValidateSearchOperator_AcceptsValidOperators(t *testing.T) {
	t.Parallel()
	assert.NoError(t, validateSearchOperator(search.OperatorAND))
	assert.NoError(t, validateSearchOperator(search.OperatorOR))
}

// ---- validateSearchRelaxation ----

func TestValidateSearchRelaxation_EmptyString_IsValid(t *testing.T) {
	t.Parallel()
	cfg := &searchCommandConfig{operator: search.OperatorAND}
	assert.NoError(t, validateSearchRelaxation(cfg))
}

func TestValidateSearchRelaxation_WithOROperator_ReturnsError(t *testing.T) {
	t.Parallel()
	cfg := &searchCommandConfig{operator: search.OperatorOR, relaxation: ">2"}
	require.Error(t, validateSearchRelaxation(cfg))
}

func TestValidateSearchRelaxation_ValidFormat_ParsesCorrectly(t *testing.T) {
	t.Parallel()

	// Arrange
	cfg := &searchCommandConfig{operator: search.OperatorAND, relaxation: ">2"}

	// Act
	err := validateSearchRelaxation(cfg)

	// Assert
	require.NoError(t, err)
	assert.True(t, cfg.relaxationEnabled)
	assert.Equal(t, 2, cfg.relaxationMin)
}

func TestValidateSearchRelaxation_MissingAngleBracket_ReturnsError(t *testing.T) {
	t.Parallel()
	cfg := &searchCommandConfig{operator: search.OperatorAND, relaxation: "2"}
	require.Error(t, validateSearchRelaxation(cfg))
}

// ---- formatVersionDate ----

func TestFormatVersionDate_ValidRFC3339_FormatsDate(t *testing.T) {
	t.Parallel()
	date := "2026-01-15T12:00:00Z"
	result := formatVersionDate(date)
	assert.NotEqual(t, date, result, "expected formatted date, got raw RFC3339 string")
	assert.Contains(t, result, "2026")
}

func TestFormatVersionDate_Invalid_PassesThrough(t *testing.T) {
	t.Parallel()
	raw := "not-a-date"
	assert.Equal(t, raw, formatVersionDate(raw))
}

// defaultConfigValues removed — use sharedconfig.DefaultFieldValue per key.

// ---- buildConfigRows ----

func TestBuildConfigRows_ReturnsAllThirteenKeys(t *testing.T) {
	t.Parallel()
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	rows := buildConfigRows(runner)
	assert.Len(t, rows, 13, "expected one row per configurable key")
}

func TestBuildConfigRows_ContainsSearchFormat(t *testing.T) {
	t.Parallel()
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	rows := buildConfigRows(runner)
	found := false
	for _, r := range rows {
		if r.key == "search.format" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected search.format in config rows")
}

// ---- validateSearchInput ----

func TestValidateSearchInput_NoQueryAndNoFilters_ReturnsError(t *testing.T) {
	t.Parallel()
	err := validateSearchInput("", nil, nil, []string{"idx", "search"})
	require.Error(t, err)
}

func TestValidateSearchInput_WithQuery_NoError(t *testing.T) {
	t.Parallel()
	assert.NoError(t, validateSearchInput("foo", nil, nil, nil))
}

func TestValidateSearchInput_ExtFilterAlone_IsValid(t *testing.T) {
	t.Parallel()
	assert.NoError(t, validateSearchInput("", nil, []string{"go"}, nil))
}

// ---- currentDirTilde ----

func TestCurrentDirTilde_ReturnsNonEmpty(t *testing.T) {
	t.Parallel()
	result := currentDirTilde()
	assert.NotEmpty(t, result)
}

// ---- searchCommandConfig.options ----

func TestSearchCommandConfig_Options_MapsAllFields(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		cfg   searchCommandConfig
		check func(t *testing.T, opts search.Options)
	}{
		{
			"maps format",
			searchCommandConfig{format: search.OutputJSON, operator: search.OperatorAND},
			func(t *testing.T, opts search.Options) { assert.Equal(t, search.OutputJSON, opts.Format) },
		},
		{
			"maps path query",
			searchCommandConfig{format: search.OutputText, operator: search.OperatorAND, pathQueries: []string{"internal/core"}},
			func(t *testing.T, opts search.Options) { assert.Equal(t, "internal/core", opts.PathQuery) },
		},
		{
			"maps extension query",
			searchCommandConfig{format: search.OutputText, operator: search.OperatorAND, extensionQueries: []string{"go"}},
			func(t *testing.T, opts search.Options) { assert.Equal(t, "go", opts.ExtensionQuery) },
		},
		{
			"maps operator",
			searchCommandConfig{format: search.OutputText, operator: search.OperatorOR},
			func(t *testing.T, opts search.Options) { assert.Equal(t, search.OperatorOR, opts.Operator) },
		},
		{
			"maps matchesOnly",
			searchCommandConfig{format: search.OutputText, operator: search.OperatorAND, matchesOnly: true},
			func(t *testing.T, opts search.Options) { assert.True(t, opts.MatchesOnly) },
		},
		{
			"maps legacyMatchesOnly",
			searchCommandConfig{format: search.OutputText, operator: search.OperatorAND, legacyMatchesOnly: true},
			func(t *testing.T, opts search.Options) { assert.True(t, opts.MatchesOnly) },
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			opts := tc.cfg.options()
			tc.check(t, opts)
		})
	}
}

// ---- showConfigBanner / writeConfigDetails / printConfigNoFileMessage / printConfigTable ----

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	return buf.String()
}

func TestShowConfigBanner_NoFile_PrintsNothing(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	// configFilePath is empty → showConfigBanner should print nothing
	output := captureStdout(t, func() { runner.showConfigBanner() })
	assert.Empty(t, output)
}

func TestShowConfigBanner_WithFile_PrintsPath(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).
		WithConfig(config.DefaultIdxConfig(), "/project/.idx.yml", nil)
	output := captureStdout(t, func() { runner.showConfigBanner() })
	assert.Contains(t, output, ".idx.yml")
}

func TestShowConfigBanner_WithOverrides_MentionsCount(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).
		WithConfig(config.DefaultIdxConfig(), "/project/.idx.yml", []string{"search.format"})
	output := captureStdout(t, func() { runner.showConfigBanner() })
	assert.Contains(t, output, "1")
}

func TestWriteConfigDetails_NoFilePath_PrintsNoFileMessage(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	output := captureStdout(t, func() {
		err := runner.writeConfigDetails()
		require.NoError(t, err)
	})
	assert.Contains(t, output, "No .idx.yml")
}

func TestWriteConfigDetails_WithFilePath_PrintsTable(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).
		WithConfig(config.DefaultIdxConfig(), "/project/.idx.yml", []string{"search.format"})
	output := captureStdout(t, func() {
		err := runner.writeConfigDetails()
		require.NoError(t, err)
	})
	assert.Contains(t, output, "search.format")
}

func TestPrintConfigNoFileMessage_ContainsTipOrPath(t *testing.T) {
	output := captureStdout(t, printConfigNoFileMessage)
	hasTip := strings.Contains(output, "Tip:") || strings.Contains(output, "tip") || strings.Contains(output, ".idx.yml")
	assert.True(t, hasTip, "expected tip in no-file message, got %q", output)
}

func TestPrintConfigTable_WithOverride_ShowsSource(t *testing.T) {
	rows := []configRow{
		{key: "search.format", value: "json", defaultValue: "text"},
	}
	overrideSet := map[string]bool{"search.format": true}
	output := captureStdout(t, func() { printConfigTable(rows, overrideSet) })
	assert.Contains(t, output, "search.format")
}

func TestPrintConfigTable_WithoutOverride_ShowsDefault(t *testing.T) {
	rows := []configRow{
		{key: "search.format", value: "text", defaultValue: "text"},
	}
	output := captureStdout(t, func() { printConfigTable(rows, map[string]bool{}) })
	assert.Contains(t, output, "search.format")
}

// ---- isIgnorableServerStopError ----

func TestIsIgnorableServerStopError_IgnorableMessages_ReturnTrue(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		msg  string
	}{
		{"not running", "server not running"},
		{"state not found", "state not found"},
		{"path not found", "path not found"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.True(t, isIgnorableServerStopError(errors.New(tc.msg)))
		})
	}
}

func TestIsIgnorableServerStopError_PermissionDenied_ReturnsFalse(t *testing.T) {
	t.Parallel()
	assert.False(t, isIgnorableServerStopError(errors.New("permission denied")))
}

// ---- WithSkillsCommand / WithReadCommand ----

type stubSkillsCommand struct{}

func (s stubSkillsCommand) Install(editor string) error { return nil }

type stubReadCommand struct{}

func (s stubReadCommand) RunWithOptions(filePath string, fromLine, toLine int) error { return nil }

func TestWithSkillsCommand_SetsField(t *testing.T) {
	t.Parallel()
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).
		WithSkillsCommand(stubSkillsCommand{})
	assert.NotNil(t, runner.skillsCommand)
}

func TestWithReadCommand_SetsField(t *testing.T) {
	t.Parallel()
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).
		WithReadCommand(stubReadCommand{})
	assert.NotNil(t, runner.readCommand)
}

// ---- InitCommandAdapter ----

type stubInitRunner struct{ runErr error }

func (s stubInitRunner) Run() error { return s.runErr }

func TestInitCommandAdapter_RunFromPath_ChangesToProjectDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	adapter := NewInitCommandAdapter(stubInitRunner{}, nil)
	assert.NoError(t, adapter.RunFromPath(dir))
}

func TestInitCommandAdapter_RunFromPath_InvalidDir_ReturnsError(t *testing.T) {
	t.Parallel()
	adapter := NewInitCommandAdapter(stubInitRunner{}, nil)
	err := adapter.RunFromPath("/nonexistent/path/xyz")
	require.Error(t, err)
}

func TestInitCommandAdapter_RunFromPath_PropagatesRunError(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	adapter := NewInitCommandAdapter(stubInitRunner{runErr: errors.New("index failed")}, nil)

	// Act
	err := adapter.RunFromPath(dir)

	// Assert
	require.Error(t, err)
	assert.ErrorContains(t, err, "index failed")
}

// ---- renderVersionOutput ----

func TestRenderVersionOutput_ContainsVersionAndDate(t *testing.T) {
	t.Parallel()
	output := renderVersionOutput("v9.9.9", "2026-01-01T00:00:00Z")
	assert.Contains(t, output, "v9.9.9")
	assert.Contains(t, output, "2026")
}
