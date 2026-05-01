package tui_test

import (
	"testing"

	"idx/internal/adapters/handlers/tui"
	"idx/internal/core/domain"
	"idx/internal/core/services/indexing"
)

func TestNewInspectRunnerImplementsPort(t *testing.T) {
	runner := tui.NewInspectRunner()
	if runner == nil {
		t.Fatal("expected NewInspectRunner to return non-nil runner")
	}
}

func TestInspectRunnerRunUsesTestHook(t *testing.T) {
	called := false
	indexing.SetRunInspectTUITestHook(func(_ *domain.InvertedIndex) error {
		called = true
		return nil
	})
	defer indexing.SetRunInspectTUITestHook(nil)

	runner := tui.NewInspectRunner()
	if err := runner.Run(domain.NewInvertedIndex()); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !called {
		t.Fatal("expected inspect TUI test hook to be invoked via runner.Run")
	}
}
