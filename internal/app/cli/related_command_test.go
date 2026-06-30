package cli

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"idx/internal/features/related"
)

// --- newRelatedCommand ---

func TestRelatedCommand_RequiresExactlyOneArg(t *testing.T) {
	t.Parallel()

	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	cmd := runner.newRelatedCommand()
	cmd.SetArgs([]string{})

	require.Error(t, cmd.Execute())
}

func TestRelatedCommand_NilRelatedCommand_ReturnsError(t *testing.T) {
	t.Parallel()

	// relatedCommand is nil by default
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	cmd := runner.newRelatedCommand()
	err := cmd.RunE(cmd, []string{"internal/features/search/service.go"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not available")
}

func TestRelatedCommand_DelegatesToRelatedCommand(t *testing.T) {
	t.Parallel()

	stub := &stubRelatedRunner{}
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).WithRelatedCommand(stub)
	cmd := runner.newRelatedCommand()

	require.NoError(t, cmd.RunE(cmd, []string{"internal/features/search/service.go"}))
	assert.True(t, stub.called)
}

func TestRelatedCommand_PropagatesRunError(t *testing.T) {
	t.Parallel()

	stub := &errRelatedRunner{err: errors.New("related failed")}
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).WithRelatedCommand(stub)
	cmd := runner.newRelatedCommand()

	require.Error(t, cmd.RunE(cmd, []string{"internal/features/search/service.go"}))
}

func TestRelatedCommand_LimitFlag_Registered(t *testing.T) {
	t.Parallel()

	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	cmd := runner.newRelatedCommand()

	assert.NotNil(t, cmd.Flags().Lookup("limit"), "expected --limit flag to be registered")
}

func TestRelatedCommand_CompactFlag_Registered(t *testing.T) {
	t.Parallel()

	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	cmd := runner.newRelatedCommand()

	assert.NotNil(t, cmd.Flags().Lookup("compact"), "expected --compact flag to be registered")
}

func TestRelatedCommand_FormatFlag_Registered(t *testing.T) {
	t.Parallel()

	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	cmd := runner.newRelatedCommand()

	assert.NotNil(t, cmd.Flags().Lookup("format"), "expected --format flag to be registered")
}

func TestRelatedCommand_SinceFlag_Registered(t *testing.T) {
	t.Parallel()

	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	cmd := runner.newRelatedCommand()

	assert.NotNil(t, cmd.Flags().Lookup("since"), "expected --since flag to be registered")
}

func TestRelatedCommand_PassesFilePathToRunner(t *testing.T) {
	t.Parallel()

	stub := &captureRelatedRunner{}
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).WithRelatedCommand(stub)
	cmd := runner.newRelatedCommand()

	require.NoError(t, cmd.RunE(cmd, []string{"internal/features/search/service.go"}))
	assert.Equal(t, "internal/features/search/service.go", stub.filePath)
}

// --- stubs ---

type stubRelatedRunner struct{ called bool }

func (s *stubRelatedRunner) Run(_ string, _ related.Options) error {
	s.called = true
	return nil
}

type errRelatedRunner struct{ err error }

func (e *errRelatedRunner) Run(_ string, _ related.Options) error { return e.err }

type captureRelatedRunner struct {
	filePath string
	opts     related.Options
}

func (c *captureRelatedRunner) Run(filePath string, opts related.Options) error {
	c.filePath = filePath
	c.opts = opts
	return nil
}
