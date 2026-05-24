package cli

import (
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
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, nil)
	cmd := runner.newWatchCommand()
	if cmd.Flags().Lookup("show-updated-files") == nil {
		t.Fatal("expected --show-updated-files flag")
	}
}

func TestWatchCommandHasDebounceFlag(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, nil)
	cmd := runner.newWatchCommand()
	if cmd.Flags().Lookup("debounce") == nil {
		t.Fatal("expected --debounce flag")
	}
}

// ---- newRootCommand ----

func TestNewRootCommandBuildsWithoutPanic(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, nil)
	cmd := runner.newRootCommand()
	if cmd == nil {
		t.Fatal("expected non-nil root command")
	}
}

func TestNewRootCommandHasQuietFlag(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, nil)
	cmd := runner.newRootCommand()
	if cmd.PersistentFlags().Lookup("quiet") == nil {
		t.Fatal("expected --quiet persistent flag")
	}
}

// ---- disableDaemonForDestroy ----

func TestDisableDaemonForDestroySuccessReturnsNil(t *testing.T) {
	svc := &stubDaemonService{}
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, svc)
	if err := runner.disableDaemonForDestroy(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDisableDaemonForDestroyIgnoresDaemonNotInitialized(t *testing.T) {
	svc := &stubDaemonService{disableErr: errors.New("daemon not initialized")}
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, svc)
	if err := runner.disableDaemonForDestroy(); err != nil {
		t.Fatalf("expected daemon-not-initialized error to be ignored, got %v", err)
	}
}

func TestDisableDaemonForDestroyIgnoresNotBeingMonitored(t *testing.T) {
	svc := &stubDaemonService{disableErr: errors.New("project not being monitored")}
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, svc)
	if err := runner.disableDaemonForDestroy(); err != nil {
		t.Fatalf("expected not-monitored error to be ignored, got %v", err)
	}
}

func TestDisableDaemonForDestroyIgnoresNoProjectsActive(t *testing.T) {
	svc := &stubDaemonService{disableErr: errors.New("no projects active")}
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, svc)
	if err := runner.disableDaemonForDestroy(); err != nil {
		t.Fatalf("expected no-projects-active error to be ignored, got %v", err)
	}
}

func TestDisableDaemonForDestroyPropagatesRealError(t *testing.T) {
	svc := &stubDaemonService{disableErr: errors.New("permission denied")}
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, svc)
	if err := runner.disableDaemonForDestroy(); err == nil {
		t.Fatal("expected permission denied error to propagate")
	}
}

// ---- newSyncCommand / newInitCommand ----

func TestNewSyncCommandBuildsWithoutPanic(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, nil)
	if runner.newSyncCommand() == nil {
		t.Fatal("expected non-nil sync command")
	}
}

func TestNewInitCommandBuildsWithoutPanic(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, nil)
	if runner.newInitCommand() == nil {
		t.Fatal("expected non-nil init command")
	}
}

// ---- newStatusCommand ----

func TestNewStatusCommandHasProfileFlag(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, nil)
	cmd := runner.newStatusCommand()
	if cmd.Flags().Lookup("profile") == nil {
		t.Fatal("expected --profile flag on status command")
	}
}

// newRunnerWithStubIndex returns a runner whose indexCommand is non-nil but never called.
func newRunnerWithStubIndex() CommandRunner {
	return NewCommandRunner([]string{"idx"}, &stubIndexCommand{}, nil, nil, nil)
}

type stubIndexCommand struct{}

func (s *stubIndexCommand) Run() error                                            { return nil }
func (s *stubIndexCommand) Sync() error                                           { return nil }
func (s *stubIndexCommand) Status() error                                         { return nil }
func (s *stubIndexCommand) Inspect(_ string) error                                { return nil }
func (s *stubIndexCommand) Watch(_ bool, _ time.Duration) error                   { return errors.New("watch not expected in test") }
