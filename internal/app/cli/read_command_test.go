package cli

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- newReadCommand ----

func TestReadCommand_RequiresExactlyOneArg(t *testing.T) {
	t.Parallel()
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	cmd := runner.newReadCommand()
	cmd.SetArgs([]string{})
	require.Error(t, cmd.Execute())
}

func TestReadCommand_NilReadCommand_ReturnsError(t *testing.T) {
	t.Parallel()
	// readCommand is nil by default
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	cmd := runner.newReadCommand()
	err := cmd.RunE(cmd, []string{"/some/file"})
	require.Error(t, err)
}

func TestReadCommand_DelegatesToReadCommand(t *testing.T) {
	t.Parallel()
	stub := &stubReadCommand{}
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).
		WithReadCommand(stub)
	cmd := runner.newReadCommand()
	assert.NoError(t, cmd.RunE(cmd, []string{"/some/file"}))
}

func TestReadCommand_PropagatesReadError(t *testing.T) {
	t.Parallel()
	stub := &errReadCommand{err: errors.New("read failed")}
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).
		WithReadCommand(stub)
	cmd := runner.newReadCommand()
	require.Error(t, cmd.RunE(cmd, []string{"/some/file"}))
}

func TestReadCommand_HasFromAndToFlags(t *testing.T) {
	t.Parallel()
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	cmd := runner.newReadCommand()
	assert.NotNil(t, cmd.Flags().Lookup("from"), "expected --from flag to be registered")
	assert.NotNil(t, cmd.Flags().Lookup("to"), "expected --to flag to be registered")
}

type errReadCommand struct{ err error }

func (e *errReadCommand) RunWithOptions(_ string, _, _ int) error { return e.err }
