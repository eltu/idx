package tui_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"idx/internal/app/tui"
	"idx/internal/features/indexing"
)

func TestNewInspectRunner_ImplementsPort(t *testing.T) {
	t.Parallel()
	runner := tui.NewInspectRunner()
	require.NotNil(t, runner)
}

func TestInspectRunner_Run_UsesTestHook(t *testing.T) {
	t.Parallel()

	// Arrange
	called := false
	tui.SetRunInspectTUITestHook(func(_ *indexing.InvertedIndex) error {
		called = true
		return nil
	})
	defer tui.SetRunInspectTUITestHook(nil)

	// Act
	runner := tui.NewInspectRunner()
	err := runner.Run(indexing.NewInvertedIndex())

	// Assert
	require.NoError(t, err)
	assert.True(t, called, "expected inspect TUI test hook to be invoked via runner.Run")
}
