package tui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"idx/internal/app/tui"
)

func TestNewInitProgressRunner_IsNotNil(t *testing.T) {
	t.Parallel()
	runner := tui.NewInitProgressRunner()
	require.NotNil(t, runner)
}

func TestInitProgressRunner_Context_IsNotNil(t *testing.T) {
	t.Parallel()
	runner := tui.NewInitProgressRunner()
	assert.NotNil(t, runner.Context())
}

func TestInitProgressRunner_SetQuiet_SilencesStartCounting(t *testing.T) {
	t.Parallel()
	runner := tui.NewInitProgressRunner()
	runner.SetQuiet(true)
	// StartCounting should be a no-op when quiet — no goroutine or program started
	runner.StartCounting()
	// If StartCounting panics or hangs, the test will fail/timeout
}

func TestInitProgressRunner_Quiet_MethodsAreNoops(t *testing.T) {
	t.Parallel()
	runner := tui.NewInitProgressRunner()
	runner.SetQuiet(true)
	runner.StartCounting()

	// All methods below should be no-ops that neither block nor panic
	runner.SetTotal(100)
	runner.IncrementDir("/some/dir")
	runner.Finish()
}

func TestInitProgressRunner_NilProgram_MethodsAreNoops(t *testing.T) {
	t.Parallel()
	runner := tui.NewInitProgressRunner()
	// program is nil when StartCounting is not called (or quiet)
	runner.SetTotal(50)          // should not block or panic
	runner.IncrementDir("/path") // should not block or panic
	runner.Finish()              // should not block or panic
}
