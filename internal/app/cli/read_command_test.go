package cli

import (
	"errors"
	"strings"
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

// ---- --start / --end flags ----

func TestReadCommand_HasStartAndEndFlags_Registered(t *testing.T) {
	t.Parallel()

	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	cmd := runner.newReadCommand()
	assert.NotNil(t, cmd.Flags().Lookup("start"), "expected --start flag to be registered")
	assert.NotNil(t, cmd.Flags().Lookup("end"), "expected --end flag to be registered")
}

func TestReadCommand_StartShorthand_RegisteredAsS(t *testing.T) {
	t.Parallel()

	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	cmd := runner.newReadCommand()
	assert.NotNil(t, cmd.Flags().ShorthandLookup("s"), "expected -s shorthand for --start")
}

func TestReadCommand_EndShorthand_RegisteredAsE(t *testing.T) {
	t.Parallel()

	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	cmd := runner.newReadCommand()
	assert.NotNil(t, cmd.Flags().ShorthandLookup("e"), "expected -e shorthand for --end")
}

func TestReadCommand_DeprecatedFromFlag_StillRegistered(t *testing.T) {
	t.Parallel()

	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	cmd := runner.newReadCommand()
	assert.NotNil(t, cmd.Flags().Lookup("from"), "deprecated --from must remain registered for backward compat")
}

func TestReadCommand_DeprecatedToFlag_StillRegistered(t *testing.T) {
	t.Parallel()

	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	cmd := runner.newReadCommand()
	assert.NotNil(t, cmd.Flags().Lookup("to"), "deprecated --to must remain registered for backward compat")
}

func TestReadCommand_StartPassedToService_CorrectValue(t *testing.T) {
	t.Parallel()

	// Arrange
	stub := &captureReadCommand{}
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).WithReadCommand(stub)
	cmd := runner.newReadCommand()

	// Act
	require.NoError(t, cmd.Flags().Set("start", "10"))
	require.NoError(t, cmd.RunE(cmd, []string{"/some/file"}))

	// Assert
	assert.Equal(t, 10, stub.fromLine)
}

func TestReadCommand_EndPassedToService_CorrectValue(t *testing.T) {
	t.Parallel()

	// Arrange
	stub := &captureReadCommand{}
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).WithReadCommand(stub)
	cmd := runner.newReadCommand()

	// Act
	require.NoError(t, cmd.Flags().Set("end", "20"))
	require.NoError(t, cmd.RunE(cmd, []string{"/some/file"}))

	// Assert
	assert.Equal(t, 20, stub.toLine)
}

// ---- aliases ----

func TestReadCommand_OpenAlias_Registered(t *testing.T) {
	t.Parallel()

	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	cmd := runner.newReadCommand()
	assert.Contains(t, cmd.Aliases, "open")
}

func TestReadCommand_CatAlias_Registered(t *testing.T) {
	t.Parallel()

	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	cmd := runner.newReadCommand()
	assert.Contains(t, cmd.Aliases, "cat")
}

func TestReadCommand_HasLongDescription(t *testing.T) {
	t.Parallel()

	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	cmd := runner.newReadCommand()
	assert.NotEmpty(t, cmd.Long)
}

// ---- --compact flag ----

func TestReadCommand_CompactFlag_Registered(t *testing.T) {
	t.Parallel()

	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	cmd := runner.newReadCommand()
	assert.NotNil(t, cmd.Flags().Lookup("compact"), "expected --compact flag to be registered")
}

func TestReadCommand_CompactFlag_DefaultIsFalse(t *testing.T) {
	t.Parallel()

	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	cmd := runner.newReadCommand()
	flag := cmd.Flags().Lookup("compact")
	require.NotNil(t, flag)
	assert.Equal(t, "false", flag.DefValue)
}

func TestReadCommand_WithoutCompact_WritesHeader(t *testing.T) {
	t.Parallel()

	// Arrange
	stub := &captureReadCommand{}
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).WithReadCommand(stub)
	cmd := runner.newReadCommand()
	var buf strings.Builder
	cmd.SetOut(&buf)

	// Act
	require.NoError(t, cmd.RunE(cmd, []string{"/some/file.go"}))

	// Assert
	assert.Contains(t, buf.String(), "/some/file.go", "expected header to contain file path when --compact is not set")
}

func TestReadCommand_WithCompact_SkipsHeader(t *testing.T) {
	t.Parallel()

	// Arrange
	stub := &captureReadCommand{}
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).WithReadCommand(stub)
	cmd := runner.newReadCommand()
	var buf strings.Builder
	cmd.SetOut(&buf)

	// Act
	require.NoError(t, cmd.Flags().Set("compact", "true"))
	require.NoError(t, cmd.RunE(cmd, []string{"/some/file.go"}))

	// Assert
	assert.Empty(t, buf.String(), "expected no header output when --compact is set")
}

type captureReadCommand struct {
	fromLine int
	toLine   int
}

func (c *captureReadCommand) RunWithOptions(_ string, from, to int) error {
	c.fromLine = from
	c.toLine = to
	return nil
}
