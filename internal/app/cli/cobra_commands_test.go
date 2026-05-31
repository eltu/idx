package cli

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- addCommandToGroup ----

func TestAddCommandToGroup_SetsGroupID(t *testing.T) {
	t.Parallel()

	// Arrange
	parent := &cobra.Command{Use: "parent"}
	parent.AddGroup(&cobra.Group{ID: "mygroup", Title: "My Group"})
	child := &cobra.Command{Use: "child"}

	// Act
	addCommandToGroup(parent, "mygroup", child)

	// Assert
	assert.Equal(t, "mygroup", child.GroupID)
}

func TestAddCommandToGroup_MultipleCommands_AllGetGroupID(t *testing.T) {
	t.Parallel()

	// Arrange
	parent := &cobra.Command{Use: "parent"}
	parent.AddGroup(&cobra.Group{ID: "g", Title: "G"})
	a := &cobra.Command{Use: "a"}
	b := &cobra.Command{Use: "b"}

	// Act
	addCommandToGroup(parent, "g", a, b)

	// Assert
	assert.Equal(t, "g", a.GroupID)
	assert.Equal(t, "g", b.GroupID)
	assert.Len(t, parent.Commands(), 2)
}

// ---- newRootCommand ----

func TestNewRootCommand_BuildsWithoutPanic(t *testing.T) {
	t.Parallel()
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	cmd := runner.newRootCommand()
	require.NotNil(t, cmd)
}

func TestNewRootCommand_HasQuietFlag(t *testing.T) {
	t.Parallel()
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	cmd := runner.newRootCommand()
	assert.NotNil(t, cmd.PersistentFlags().Lookup("quiet"), "expected --quiet persistent flag")
}

// ---- stopServerForDestroy ----

type stubServerManager struct {
	stopErr error
}

func (s *stubServerManager) Start(_ string) error  { return nil }
func (s *stubServerManager) Stop(_ string) error   { return s.stopErr }
func (s *stubServerManager) Status(_ string) error { return nil }

func TestStopServerForDestroy_SuccessfulStop_ReturnsNil(t *testing.T) {
	t.Parallel()
	mgr := &stubServerManager{}
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).WithServerManager(mgr)
	assert.NoError(t, runner.stopServerForDestroy())
}

func TestStopServerForDestroy_IgnorableErrors_Swallowed(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		msg  string
	}{
		{"not running", "server not running"},
		{"state not found", "state not found"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mgr := &stubServerManager{stopErr: errors.New(tc.msg)}
			runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).WithServerManager(mgr)
			assert.NoError(t, runner.stopServerForDestroy())
		})
	}
}

func TestStopServerForDestroy_PermissionDenied_Propagates(t *testing.T) {
	t.Parallel()
	mgr := &stubServerManager{stopErr: errors.New("permission denied")}
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).WithServerManager(mgr)
	require.Error(t, runner.stopServerForDestroy())
}

func TestStopServerForDestroy_NilManager_ReturnsNil(t *testing.T) {
	t.Parallel()
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	assert.NoError(t, runner.stopServerForDestroy())
}

// ---- newSyncCommand / newInitCommand ----

func TestNewSyncCommand_BuildsWithoutPanic(t *testing.T) {
	t.Parallel()
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	assert.NotNil(t, runner.newSyncCommand())
}

func TestNewInitCommand_BuildsWithoutPanic(t *testing.T) {
	t.Parallel()
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	assert.NotNil(t, runner.newInitCommand())
}

// ---- newStatusCommand ----

func TestNewStatusCommand_HasProfileFlag(t *testing.T) {
	t.Parallel()
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	cmd := runner.newStatusCommand()
	assert.NotNil(t, cmd.Flags().Lookup("profile"), "expected --profile flag on status command")
}

// newRunnerWithStubIndex returns a runner whose indexCommand is non-nil but never called.
func newRunnerWithStubIndex() CommandRunner {
	return NewCommandRunner([]string{"idx"}, &stubIndexCommand{}, nil, nil)
}

type stubIndexCommand struct{}

func (s *stubIndexCommand) Run() error             { return nil }
func (s *stubIndexCommand) Sync() error            { return nil }
func (s *stubIndexCommand) Status() error          { return nil }
func (s *stubIndexCommand) Inspect(_ string) error { return nil }
func (s *stubIndexCommand) Watch(_ bool, _ time.Duration) error {
	return errors.New("watch not expected in test")
}
func (s *stubIndexCommand) WatchWithContext(_ context.Context, _ time.Duration) error {
	return errors.New("watch not expected in test")
}

// stubDestroyCommand implements Runner for newDestroyCommand RunE tests.
type stubDestroyCommand struct{}

func (s *stubDestroyCommand) Run() error { return nil }

// ---- newInitCommand RunE ----

func TestNewInitCommand_RunE_CallsRun(t *testing.T) {
	t.Parallel()
	runner := newRunnerWithStubIndex()
	cmd := runner.newInitCommand()
	assert.NoError(t, cmd.RunE(cmd, []string{}))
}

// ---- newStatusCommand RunE ----

func TestNewStatusCommand_RunE_CallsStatus(t *testing.T) {
	t.Parallel()
	runner := newRunnerWithStubIndex()
	cmd := runner.newStatusCommand()
	assert.NoError(t, cmd.RunE(cmd, []string{}))
}

// ---- newInspectCommand RunE ----

func TestNewInspectCommand_RunE_CallsInspect(t *testing.T) {
	t.Parallel()
	runner := newRunnerWithStubIndex()
	cmd := runner.newInspectCommand()
	assert.NoError(t, cmd.RunE(cmd, []string{}))
}

// ---- newDestroyCommand RunE ----

func TestNewDestroyCommand_RunE_CallsDestroy(t *testing.T) {
	t.Parallel()
	runner := NewCommandRunner([]string{"idx"}, nil, &stubDestroyCommand{}, nil)
	cmd := runner.newDestroyCommand()
	assert.NoError(t, cmd.RunE(cmd, []string{}))
}

// ---- newStatusCommand RunE branches ----

// stubIndexCommandWithStatusContext additionally implements StatusWithContext.
type stubIndexCommandWithStatusContext struct{ stubIndexCommand }

func (s *stubIndexCommandWithStatusContext) StatusWithContext(_ string, _ []string) error {
	return nil
}

func TestNewStatusCommand_RunE_CallsStatusWithContext(t *testing.T) {
	t.Parallel()
	runner := NewCommandRunner([]string{"idx"}, &stubIndexCommandWithStatusContext{}, nil, nil)
	cmd := runner.newStatusCommand()
	assert.NoError(t, cmd.RunE(cmd, []string{}))
}

// stubIndexCommandWithProfile additionally implements StatusWithProfile.
type stubIndexCommandWithProfile struct{ stubIndexCommand }

func (s *stubIndexCommandWithProfile) StatusWithProfile(_ bool) error { return nil }

func TestNewStatusCommand_RunEWithProfileFlag_CallsStatusWithProfile(t *testing.T) {
	t.Parallel()
	runner := NewCommandRunner([]string{"idx"}, &stubIndexCommandWithProfile{}, nil, nil)
	cmd := runner.newStatusCommand()
	_ = cmd.ParseFlags([]string{"--profile"})
	assert.NoError(t, cmd.RunE(cmd, []string{}))
}

// ---- newInspectCommand Args validation ----

func TestNewInspectCommand_Args_ValidatesEmptyArgs(t *testing.T) {
	t.Parallel()
	runner := newRunnerWithStubIndex()
	cmd := runner.newInspectCommand()
	assert.NoError(t, cmd.Args(cmd, []string{}))
}

func TestNewInspectCommand_RunE_MultipleArgs_ReturnsError(t *testing.T) {
	t.Parallel()
	runner := newRunnerWithStubIndex()
	cmd := runner.newInspectCommand()
	require.Error(t, cmd.RunE(cmd, []string{"path1", "path2"}))
}

// ---- Long descriptions ----

func TestNewInspectCommand_HasLongDescription(t *testing.T) {
	t.Parallel()
	runner := newRunnerWithStubIndex()
	cmd := runner.newInspectCommand()
	assert.NotEmpty(t, cmd.Long)
}

func TestNewSyncCommand_HasLongDescription(t *testing.T) {
	t.Parallel()
	runner := newRunnerWithStubIndex()
	cmd := runner.newSyncCommand()
	assert.NotEmpty(t, cmd.Long)
}

// ---- Aliases ----

func TestNewSyncCommand_UpdateAlias_Registered(t *testing.T) {
	t.Parallel()
	runner := newRunnerWithStubIndex()
	cmd := runner.newSyncCommand()
	assert.Contains(t, cmd.Aliases, "update")
}

func TestNewInitCommand_HasLongDescription(t *testing.T) {
	t.Parallel()
	runner := newRunnerWithStubIndex()
	cmd := runner.newInitCommand()
	assert.NotEmpty(t, cmd.Long)
}

func TestNewDestroyCommand_HasLongDescription(t *testing.T) {
	t.Parallel()
	runner := newRunnerWithStubIndex()
	cmd := runner.newDestroyCommand()
	assert.NotEmpty(t, cmd.Long)
}
