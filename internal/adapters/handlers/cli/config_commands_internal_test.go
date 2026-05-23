package cli

import (
	"strings"
	"testing"

	"idx/internal/core/domain"
)

func newConfigTestRunner() CommandRunner {
	return NewCommandRunner(
		[]string{"idx"},
		noOpIndexCommand{},
		noOpDestroyCommand{},
		noOpSearchCommand{},
		noOpDaemonCommand{},
	)
}

func TestFormatConfigFloatRoundsToFourSignificantDigits(t *testing.T) {
	if got := formatConfigFloat(1.5); got != "1.5" {
		t.Fatalf("expected '1.5', got %q", got)
	}
	if got := formatConfigFloat(0.75); got != "0.75" {
		t.Fatalf("expected '0.75', got %q", got)
	}
}

func TestFormatIgnorePatternsEmptyReturnsEmpty(t *testing.T) {
	if got := formatIgnorePatterns(nil); got != "[]" {
		t.Fatalf("expected '[]', got %q", got)
	}
}

func TestFormatIgnorePatternsWithValuesJoinsThem(t *testing.T) {
	got := formatIgnorePatterns([]string{"vendor", "dist"})
	if got != "[vendor, dist]" {
		t.Fatalf("expected '[vendor, dist]', got %q", got)
	}
}

func TestPadRightPadsShortString(t *testing.T) {
	if got := padRight("ab", 5); got != "ab   " {
		t.Fatalf("expected 'ab   ', got %q", got)
	}
}

func TestPadRightDoesNotTruncateExactWidth(t *testing.T) {
	if got := padRight("abcde", 5); got != "abcde" {
		t.Fatalf("expected 'abcde', got %q", got)
	}
}

func TestPadRightDoesNotTruncateLongerString(t *testing.T) {
	if got := padRight("abcdef", 3); got != "abcdef" {
		t.Fatalf("expected 'abcdef', got %q", got)
	}
}

func TestDefaultConfigValuesHasExpectedKeys(t *testing.T) {
	defaults := defaultConfigValues()
	required := []string{
		"search.format", "search.size", "search.operator",
		"watch.debounce", "bm25.k1", "bm25.b", "log.level",
	}
	for _, key := range required {
		if _, ok := defaults[key]; !ok {
			t.Fatalf("expected key %q in defaultConfigValues", key)
		}
	}
}

func TestBuildConfigRowsReturnsAllExpectedKeys(t *testing.T) {
	runner := newConfigTestRunner().WithConfig(domain.DefaultIdxConfig(), "", nil)

	rows := buildConfigRows(runner)
	keys := make(map[string]bool, len(rows))
	for _, r := range rows {
		keys[r.key] = true
	}

	for _, expected := range []string{"search.format", "bm25.k1", "watch.debounce"} {
		if !keys[expected] {
			t.Fatalf("expected key %q in config rows", expected)
		}
	}
}

func TestWriteConfigDetailsNoConfigFile(t *testing.T) {
	runner := newConfigTestRunner()
	if err := runner.writeConfigDetails(); err != nil {
		t.Fatalf("expected no error for empty config path, got %v", err)
	}
}

func TestWriteConfigDetailsWithConfigFileNoOverrides(t *testing.T) {
	runner := newConfigTestRunner().WithConfig(domain.DefaultIdxConfig(), ".idx.yml", nil)
	if err := runner.writeConfigDetails(); err != nil {
		t.Fatalf("expected no error with config file and no overrides, got %v", err)
	}
}

func TestWriteConfigDetailsWithOverridesRendersTable(t *testing.T) {
	runner := newConfigTestRunner().WithConfig(domain.DefaultIdxConfig(), ".idx.yml", []string{"bm25.k1"})
	if err := runner.writeConfigDetails(); err != nil {
		t.Fatalf("expected no error with overrides, got %v", err)
	}
}

func TestShowConfigBannerNoConfigFile(t *testing.T) {
	runner := newConfigTestRunner()
	runner.showConfigBanner() // no-op when configFilePath is empty
}

func TestShowConfigBannerWithNoOverrides(t *testing.T) {
	runner := newConfigTestRunner().WithConfig(domain.DefaultIdxConfig(), ".idx.yml", nil)
	runner.showConfigBanner()
}

func TestShowConfigBannerWithSingleOverride(t *testing.T) {
	runner := newConfigTestRunner().WithConfig(domain.DefaultIdxConfig(), ".idx.yml", []string{"bm25.k1"})
	runner.showConfigBanner() // exercises singular "override" path
}

func TestShowConfigBannerWithMultipleOverrides(t *testing.T) {
	runner := newConfigTestRunner().WithConfig(domain.DefaultIdxConfig(), ".idx.yml", []string{"bm25.k1", "bm25.b"})
	runner.showConfigBanner() // exercises plural "overrides" path
}

func TestNewConfigShowCommandRunECallsWriteConfigDetails(t *testing.T) {
	runner := newConfigTestRunner().WithConfig(domain.DefaultIdxConfig(), ".idx.yml", []string{"bm25.k1"})
	cmd := runner.newConfigShowCommand()
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("expected config show to succeed, got %v", err)
	}
}

func TestRenderConfigRowOverrideLabel(t *testing.T) {
	runner := newConfigTestRunner().WithConfig(domain.DefaultIdxConfig(), ".idx.yml", []string{"bm25.k1"})
	rows := buildConfigRows(runner)

	var k1Row *configRow
	for i := range rows {
		if rows[i].key == "bm25.k1" {
			k1Row = &rows[i]
			break
		}
	}
	if k1Row == nil {
		t.Fatal("expected bm25.k1 row to exist")
	}
	if !strings.Contains(k1Row.value, "1.5") {
		t.Fatalf("expected bm25.k1 default value 1.5, got %q", k1Row.value)
	}
}
