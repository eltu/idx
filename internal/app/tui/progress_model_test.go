package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// ---- shortProgressDirName ----

func TestShortProgressDirNameShallowPathUnchanged(t *testing.T) {
	got := shortProgressDirName("foo/bar")
	if got != "foo/bar" {
		t.Fatalf("expected unchanged for 2-part path, got %q", got)
	}
}

func TestShortProgressDirNameSinglePartUnchanged(t *testing.T) {
	got := shortProgressDirName("single")
	if got != "single" {
		t.Fatalf("expected unchanged for 1-part path, got %q", got)
	}
}

func TestShortProgressDirNameDeepPathKeepsLastTwo(t *testing.T) {
	got := shortProgressDirName("a/b/c/d")
	if got != "c/d" {
		t.Fatalf("expected c/d, got %q", got)
	}
}

func TestShortProgressDirNameWindowsSlashNormalized(t *testing.T) {
	got := shortProgressDirName(`a\b\c\d`)
	if got != "c/d" {
		t.Fatalf("expected c/d for windows path, got %q", got)
	}
}

func TestShortProgressDirNameExactlyThreePartsKeepsLastTwo(t *testing.T) {
	got := shortProgressDirName("x/y/z")
	if got != "y/z" {
		t.Fatalf("expected y/z, got %q", got)
	}
}

// ---- progressPercent ----

func TestProgressPercentZeroTotalIsZero(t *testing.T) {
	m := initProgressModel{total: 0, current: 0}
	if progressPercent(m) != 0.0 {
		t.Fatalf("expected 0.0 for zero total")
	}
}

func TestProgressPercentHalf(t *testing.T) {
	m := initProgressModel{total: 10, current: 5}
	got := progressPercent(m)
	if got < 0.49 || got > 0.51 {
		t.Fatalf("expected ~0.5, got %f", got)
	}
}

func TestProgressPercentFull(t *testing.T) {
	m := initProgressModel{total: 10, current: 10}
	if progressPercent(m) != 1.0 {
		t.Fatalf("expected 1.0 for full progress")
	}
}

func TestProgressPercentCapsAtOne(t *testing.T) {
	m := initProgressModel{total: 5, current: 20}
	if progressPercent(m) != 1.0 {
		t.Fatalf("expected 1.0 cap when current > total")
	}
}

// ---- renderGradientFilled ----

func TestRenderGradientFilledZeroIsEmpty(t *testing.T) {
	got := renderGradientFilled(0)
	if got != "" {
		t.Fatalf("expected empty string for 0, got %q", got)
	}
}

func TestRenderGradientFilledOneBlock(t *testing.T) {
	got := renderGradientFilled(1)
	if !strings.Contains(got, "█") {
		t.Fatalf("expected block character in output, got %q", got)
	}
}

func TestRenderGradientFilledMultipleBlocks(t *testing.T) {
	got := renderGradientFilled(5)
	count := strings.Count(got, "█")
	if count != 5 {
		t.Fatalf("expected 5 block chars, got %d", count)
	}
}

// ---- renderInitProgressDirLine ----

func TestRenderInitProgressDirLineEmptyIsEmpty(t *testing.T) {
	got := renderInitProgressDirLine("")
	if got != "" {
		t.Fatalf("expected empty string for empty dir, got %q", got)
	}
}

func TestRenderInitProgressDirLineNonEmptyContainsDir(t *testing.T) {
	got := renderInitProgressDirLine("foo/bar")
	if !strings.Contains(got, "bar") {
		t.Fatalf("expected dir name in line, got %q", got)
	}
}

// ---- initProgressModel Update ----

func TestInitProgressModelUpdateWindowSize(t *testing.T) {
	m := newInitProgressModel(nil, nil, func() {})
	result, _ := m.Update(tea.WindowSizeMsg{Width: 120})
	updated := result.(initProgressModel)
	if updated.width != 120 {
		t.Fatalf("expected width=120, got %d", updated.width)
	}
}

func TestInitProgressModelUpdateSpinnerAdvancesFrame(t *testing.T) {
	m := newInitProgressModel(nil, nil, func() {})
	m.phase = phaseCounting
	m.spinnerIdx = 0
	result, _ := m.Update(progressSpinnerMsg{})
	updated := result.(initProgressModel)
	if updated.spinnerIdx != 1 {
		t.Fatalf("expected spinnerIdx=1, got %d", updated.spinnerIdx)
	}
}

func TestInitProgressModelUpdateSpinnerWrapsAround(t *testing.T) {
	m := newInitProgressModel(nil, nil, func() {})
	m.phase = phaseCounting
	m.spinnerIdx = len(spinnerFrames) - 1
	result, _ := m.Update(progressSpinnerMsg{})
	updated := result.(initProgressModel)
	if updated.spinnerIdx != 0 {
		t.Fatalf("expected spinnerIdx to wrap to 0, got %d", updated.spinnerIdx)
	}
}

func TestInitProgressModelUpdateSpinnerIgnoredDuringIndexing(t *testing.T) {
	progressCh := make(chan string)
	totalCh := make(chan int)
	m := newInitProgressModel(progressCh, totalCh, func() {})
	m.phase = phaseIndexing
	m.spinnerIdx = 2
	result, _ := m.Update(progressSpinnerMsg{})
	updated := result.(initProgressModel)
	if updated.spinnerIdx != 2 {
		t.Fatalf("expected spinnerIdx unchanged during indexing, got %d", updated.spinnerIdx)
	}
}

func TestInitProgressModelUpdateSetTotalSwitchesPhase(t *testing.T) {
	progressCh := make(chan string, 1)
	totalCh := make(chan int, 1)
	m := newInitProgressModel(progressCh, totalCh, func() {})
	m.phase = phaseCounting
	result, _ := m.Update(progressSetTotalMsg{total: 42})
	updated := result.(initProgressModel)
	if updated.phase != phaseIndexing {
		t.Fatal("expected phase to switch to phaseIndexing")
	}
	if updated.total != 42 {
		t.Fatalf("expected total=42, got %d", updated.total)
	}
}

func TestInitProgressModelUpdateProgressTickIncrements(t *testing.T) {
	progressCh := make(chan string, 1)
	totalCh := make(chan int, 1)
	m := newInitProgressModel(progressCh, totalCh, func() {})
	m.phase = phaseIndexing
	m.total = 10
	m.current = 3
	result, _ := m.Update(progressTickMsg{dir: "some/dir"})
	updated := result.(initProgressModel)
	if updated.current != 4 {
		t.Fatalf("expected current=4, got %d", updated.current)
	}
	if updated.lastDir != "some/dir" {
		t.Fatalf("expected lastDir=some/dir, got %q", updated.lastDir)
	}
}

func TestInitProgressModelUpdateDoneSetsTotalAsCurrent(t *testing.T) {
	m := newInitProgressModel(nil, nil, func() {})
	m.total = 10
	m.current = 7
	result, _ := m.Update(progressDoneMsg{})
	updated := result.(initProgressModel)
	if updated.current != 10 {
		t.Fatalf("expected current==total on done, got %d", updated.current)
	}
}

func TestInitProgressModelUpdateCtrlCCallsCancel(t *testing.T) {
	canceled := false
	m := newInitProgressModel(nil, nil, func() { canceled = true })
	m.Update(tea.KeyPressMsg{Text: "ctrl+c"})
	if !canceled {
		t.Fatal("expected cancelFunc to be called on ctrl+c")
	}
}

// ---- initProgressModel View ----

func TestInitProgressModelViewCountingPhase(t *testing.T) {
	m := newInitProgressModel(nil, nil, func() {})
	m.phase = phaseCounting
	view := m.View()
	_ = view // just verify it doesn't panic
}

func TestInitProgressModelViewIndexingPhaseZeroTotal(t *testing.T) {
	m := newInitProgressModel(nil, nil, func() {})
	m.phase = phaseIndexing
	m.total = 0
	view := m.View()
	_ = view
}

func TestInitProgressModelViewIndexingPhaseWithProgress(t *testing.T) {
	m := newInitProgressModel(nil, nil, func() {})
	m.phase = phaseIndexing
	m.total = 10
	m.current = 5
	m.width = 80
	view := m.View()
	_ = view
}

// ---- newInitProgressModel ----

func TestNewInitProgressModelHasChannels(t *testing.T) {
	progressCh := make(chan string)
	totalCh := make(chan int)
	_, cancel := context.WithCancel(context.Background())
	m := newInitProgressModel(progressCh, totalCh, cancel)
	if m.progressCh == nil {
		t.Fatal("expected progressCh to be set")
	}
	if m.totalCh == nil {
		t.Fatal("expected totalCh to be set")
	}
	if m.cancelFunc == nil {
		t.Fatal("expected cancelFunc to be set")
	}
}

// ---- renderInitProgressBar ----

func TestRenderInitProgressBarNarrowWidthUsesMinimum(t *testing.T) {
	m := initProgressModel{total: 10, current: 5, width: 5}
	got := renderInitProgressBar(m)
	if got == "" {
		t.Fatal("expected non-empty bar")
	}
}

func TestRenderInitProgressBarZeroWidthDefaultsTo80(t *testing.T) {
	m := initProgressModel{total: 10, current: 5, width: 0}
	got := renderInitProgressBar(m)
	if got == "" {
		t.Fatal("expected non-empty bar with default width")
	}
}
