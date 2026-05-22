package cli_test

import (
	"errors"
	"path/filepath"
	"testing"

	"idx/internal/adapters/handlers/cli"
)

type stubInitRunner struct{ err error }

func (s stubInitRunner) Run() error { return s.err }

func TestInitCommandAdapterRunFromPathReturnsErrorForNonexistentPath(t *testing.T) {
	adapter := cli.NewInitCommandAdapter(stubInitRunner{}, nil)

	err := adapter.RunFromPath(filepath.Join(t.TempDir(), "nonexistent"))
	if err == nil {
		t.Fatal("expected error for nonexistent path, got nil")
	}
}

func TestInitCommandAdapterRunFromPathDelegatesToInitService(t *testing.T) {
	adapter := cli.NewInitCommandAdapter(stubInitRunner{err: errors.New("init failed")}, nil)

	err := adapter.RunFromPath(t.TempDir())
	if err == nil {
		t.Fatal("expected error from init service, got nil")
	}
}

func TestInitCommandAdapterRunFromPathSucceeds(t *testing.T) {
	adapter := cli.NewInitCommandAdapter(stubInitRunner{}, nil)

	if err := adapter.RunFromPath(t.TempDir()); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}
