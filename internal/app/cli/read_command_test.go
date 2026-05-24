package cli

import (
	"errors"
	"testing"
)

// ---- newReadCommand ----

func TestReadCommandRequiresExactlyOneArg(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, nil)
	cmd := runner.newReadCommand()
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error when no args provided")
	}
}

func TestReadCommandReturnsErrorWhenReadCommandNil(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, nil)
	// readCommand is nil by default
	cmd := runner.newReadCommand()
	err := cmd.RunE(cmd, []string{"/some/file"})
	if err == nil {
		t.Fatal("expected error when readCommand is not configured")
	}
}

func TestReadCommandDelegatesToReadCommand(t *testing.T) {
	stub := &stubReadCommand{}
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, nil).
		WithReadCommand(stub)
	cmd := runner.newReadCommand()
	if err := cmd.RunE(cmd, []string{"/some/file"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadCommandPropagatesReadError(t *testing.T) {
	stub := &errReadCommand{err: errors.New("read failed")}
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, nil).
		WithReadCommand(stub)
	cmd := runner.newReadCommand()
	if err := cmd.RunE(cmd, []string{"/some/file"}); err == nil {
		t.Fatal("expected error to propagate from readCommand")
	}
}

func TestReadCommandHasFromAndToFlags(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, nil)
	cmd := runner.newReadCommand()
	if cmd.Flags().Lookup("from") == nil {
		t.Fatal("expected --from flag to be registered")
	}
	if cmd.Flags().Lookup("to") == nil {
		t.Fatal("expected --to flag to be registered")
	}
}

type errReadCommand struct{ err error }

func (e *errReadCommand) RunWithOptions(_ string, _, _ int) error { return e.err }
