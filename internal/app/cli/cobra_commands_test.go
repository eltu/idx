package cli

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// ---- addCommandToGroup ----

func TestAddCommandToGroupSetsGroupID(t *testing.T) {
	parent := &cobra.Command{Use: "parent"}
	parent.AddGroup(&cobra.Group{ID: "mygroup", Title: "My Group"})
	child := &cobra.Command{Use: "child"}
	addCommandToGroup(parent, "mygroup", child)
	if child.GroupID != "mygroup" {
		t.Fatalf("expected GroupID=mygroup, got %q", child.GroupID)
	}
}

func TestAddCommandToGroupMultipleCommands(t *testing.T) {
	parent := &cobra.Command{Use: "parent"}
	parent.AddGroup(&cobra.Group{ID: "g", Title: "G"})
	a := &cobra.Command{Use: "a"}
	b := &cobra.Command{Use: "b"}
	addCommandToGroup(parent, "g", a, b)
	if a.GroupID != "g" || b.GroupID != "g" {
		t.Fatalf("expected both commands to have GroupID=g")
	}
	if len(parent.Commands()) != 2 {
		t.Fatalf("expected 2 subcommands, got %d", len(parent.Commands()))
	}
}

// ---- newWatchCommand ----

func TestWatchCommandZeroDebounceReturnsError(t *testing.T) {
	runner := newRunnerWithStubIndex()
	cmd := runner.newWatchCommand()
	// Set debounce flag to 0 via args then call RunE
	cmd.SetArgs([]string{"--debounce", "0s"})
	_ = cmd.ParseFlags([]string{"--debounce", "0s"})
	err := cmd.RunE(cmd, []string{})
	if err == nil {
		t.Fatal("expected error for zero debounce")
	}
}

func TestWatchCommandNegativeDebounceReturnsError(t *testing.T) {
	runner := newRunnerWithStubIndex()
	cmd := runner.newWatchCommand()
	_ = cmd.ParseFlags([]string{"--debounce", "-1s"})
	err := cmd.RunE(cmd, []string{})
	if err == nil {
		t.Fatal("expected error for negative debounce")
	}
}

func TestWatchCommandHasShowUpdatedFilesFlag(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	cmd := runner.newWatchCommand()
	if cmd.Flags().Lookup("show-updated-files") == nil {
		t.Fatal("expected --show-updated-files flag")
	}
}

func TestWatchCommandHasDebounceFlag(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	cmd := runner.newWatchCommand()
	if cmd.Flags().Lookup("debounce") == nil {
		t.Fatal("expected --debounce flag")
	}
}

// ---- newRootCommand ----

func TestNewRootCommandBuildsWithoutPanic(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	cmd := runner.newRootCommand()
	if cmd == nil {
		t.Fatal("expected non-nil root command")
	}
}

func TestNewRootCommandHasQuietFlag(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	cmd := runner.newRootCommand()
	if cmd.PersistentFlags().Lookup("quiet") == nil {
		t.Fatal("expected --quiet persistent flag")
	}
}

// ---- stopServerForDestroy ----

type stubServerManager struct {
	stopErr error
}

func (s *stubServerManager) Start(_ string) error { return nil }
func (s *stubServerManager) Stop(_ string) error  { return s.stopErr }
func (s *stubServerManager) Status(_ string) error { return nil }

func TestStopServerForDestroySuccessReturnsNil(t *testing.T) {
	mgr := &stubServerManager{}
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).WithServerManager(mgr)
	if err := runner.stopServerForDestroy(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStopServerForDestroyIgnoresNotRunning(t *testing.T) {
	mgr := &stubServerManager{stopErr: errors.New("server not running")}
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).WithServerManager(mgr)
	if err := runner.stopServerForDestroy(); err != nil {
		t.Fatalf("expected not-running error to be ignored, got %v", err)
	}
}

func TestStopServerForDestroyIgnoresStateNotFound(t *testing.T) {
	mgr := &stubServerManager{stopErr: errors.New("state not found")}
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).WithServerManager(mgr)
	if err := runner.stopServerForDestroy(); err != nil {
		t.Fatalf("expected state-not-found error to be ignored, got %v", err)
	}
}

func TestStopServerForDestroyPropagatesRealError(t *testing.T) {
	mgr := &stubServerManager{stopErr: errors.New("permission denied")}
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).WithServerManager(mgr)
	if err := runner.stopServerForDestroy(); err == nil {
		t.Fatal("expected permission denied error to propagate")
	}
}

func TestStopServerForDestroyNoopWhenNilManager(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	if err := runner.stopServerForDestroy(); err != nil {
		t.Fatalf("unexpected error with nil server manager: %v", err)
	}
}

// ---- newSyncCommand / newInitCommand ----

func TestNewSyncCommandBuildsWithoutPanic(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	if runner.newSyncCommand() == nil {
		t.Fatal("expected non-nil sync command")
	}
}

func TestNewInitCommandBuildsWithoutPanic(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	if runner.newInitCommand() == nil {
		t.Fatal("expected non-nil init command")
	}
}

// ---- newStatusCommand ----

func TestNewStatusCommandHasProfileFlag(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	cmd := runner.newStatusCommand()
	if cmd.Flags().Lookup("profile") == nil {
		t.Fatal("expected --profile flag on status command")
	}
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

func TestNewInitCommandRunECallsRun(t *testing.T) {
	runner := newRunnerWithStubIndex()
	cmd := runner.newInitCommand()
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---- newStatusCommand RunE ----

func TestNewStatusCommandRunECallsStatus(t *testing.T) {
	runner := newRunnerWithStubIndex()
	cmd := runner.newStatusCommand()
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---- newInspectCommand RunE ----

func TestNewInspectCommandRunECallsInspect(t *testing.T) {
	runner := newRunnerWithStubIndex()
	cmd := runner.newInspectCommand()
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---- newDestroyCommand RunE ----

func TestNewDestroyCommandRunECallsDestroy(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, &stubDestroyCommand{}, nil)
	cmd := runner.newDestroyCommand()
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---- newStatusCommand RunE branches ----

// stubIndexCommandWithStatusContext additionally implements StatusWithContext.
type stubIndexCommandWithStatusContext struct{ stubIndexCommand }

func (s *stubIndexCommandWithStatusContext) StatusWithContext(_ string, _ []string) error {
	return nil
}

func TestNewStatusCommandRunECallsStatusWithContext(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, &stubIndexCommandWithStatusContext{}, nil, nil)
	cmd := runner.newStatusCommand()
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// stubIndexCommandWithProfile additionally implements StatusWithProfile.
type stubIndexCommandWithProfile struct{ stubIndexCommand }

func (s *stubIndexCommandWithProfile) StatusWithProfile(_ bool) error { return nil }

func TestNewStatusCommandRunEWithProfileFlagCallsStatusWithProfile(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, &stubIndexCommandWithProfile{}, nil, nil)
	cmd := runner.newStatusCommand()
	_ = cmd.ParseFlags([]string{"--profile"})
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---- newInspectCommand Args validation ----

func TestNewInspectCommandArgsValidatesEmptyArgs(t *testing.T) {
	runner := newRunnerWithStubIndex()
	cmd := runner.newInspectCommand()
	if err := cmd.Args(cmd, []string{}); err != nil {
		t.Fatalf("unexpected error for empty args: %v", err)
	}
}

func TestNewInspectCommandRunEWithMultipleArgsReturnsError(t *testing.T) {
	runner := newRunnerWithStubIndex()
	cmd := runner.newInspectCommand()
	if err := cmd.RunE(cmd, []string{"path1", "path2"}); err == nil {
		t.Fatal("expected error for multiple inspect arguments")
	}
}
