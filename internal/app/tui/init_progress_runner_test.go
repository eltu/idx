package tui_test

import (
	"testing"

	"idx/internal/app/tui"
)

func TestNewInitProgressRunnerIsNotNil(t *testing.T) {
	runner := tui.NewInitProgressRunner()
	if runner == nil {
		t.Fatal("expected non-nil InitProgressRunner")
	}
}

func TestInitProgressRunnerContextIsNotNil(t *testing.T) {
	runner := tui.NewInitProgressRunner()
	if runner.Context() == nil {
		t.Fatal("expected non-nil context")
	}
}

func TestInitProgressRunnerSetQuietSilencesStartCounting(t *testing.T) {
	runner := tui.NewInitProgressRunner()
	runner.SetQuiet(true)
	// StartCounting should be a no-op when quiet — no goroutine or program started
	runner.StartCounting()
	// If StartCounting panics or hangs, the test will fail/timeout
}

func TestInitProgressRunnerSetTotalQuietIsNoop(t *testing.T) {
	runner := tui.NewInitProgressRunner()
	runner.SetQuiet(true)
	runner.StartCounting()
	runner.SetTotal(100) // should not block or panic
}

func TestInitProgressRunnerIncrementDirQuietIsNoop(t *testing.T) {
	runner := tui.NewInitProgressRunner()
	runner.SetQuiet(true)
	runner.StartCounting()
	runner.IncrementDir("/some/dir") // should not block or panic
}

func TestInitProgressRunnerFinishQuietIsNoop(t *testing.T) {
	runner := tui.NewInitProgressRunner()
	runner.SetQuiet(true)
	runner.StartCounting()
	runner.Finish() // should not block or panic
}

func TestInitProgressRunnerSetTotalNilProgramIsNoop(t *testing.T) {
	runner := tui.NewInitProgressRunner()
	// program is nil when StartCounting is not called (or quiet)
	runner.SetTotal(50) // should not block or panic
}

func TestInitProgressRunnerIncrementDirNilProgramIsNoop(t *testing.T) {
	runner := tui.NewInitProgressRunner()
	runner.IncrementDir("/path") // should not block or panic
}

func TestInitProgressRunnerFinishNilProgramIsNoop(t *testing.T) {
	runner := tui.NewInitProgressRunner()
	runner.Finish() // should not block or panic
}
