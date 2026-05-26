package cli

import (
	"errors"
	"testing"
)

type stubDaemonService struct {
	enableErr   error
	disableErr  error
	statusErr   error
	lastEnable  string
	lastDisable string
}

func (s *stubDaemonService) Enable(projectPath string) error {
	s.lastEnable = projectPath
	return s.enableErr
}

func (s *stubDaemonService) Disable(projectPath string) error {
	s.lastDisable = projectPath
	return s.disableErr
}

func (s *stubDaemonService) Status() error {
	return s.statusErr
}

// ---- newDaemonCommand ----

func TestNewDaemonCommandHasThreeSubcommands(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, nil)
	cmd := runner.newDaemonCommand()
	if len(cmd.Commands()) != 3 {
		t.Fatalf("expected 3 subcommands, got %d", len(cmd.Commands()))
	}
}

// ---- newDaemonEnableCommand ----

func TestDaemonEnableRequiresOneArg(t *testing.T) {
	svc := &stubDaemonService{}
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, svc)
	cmd := runner.newDaemonEnableCommand()
	cmd.SetArgs([]string{})
	err := cmd.Args(cmd, []string{})
	if err == nil {
		t.Fatal("expected error when enable called with no args")
	}
}

func TestDaemonEnableRejectsMultipleArgs(t *testing.T) {
	svc := &stubDaemonService{}
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, svc)
	cmd := runner.newDaemonEnableCommand()
	err := cmd.Args(cmd, []string{"a", "b"})
	if err == nil {
		t.Fatal("expected error when enable called with two args")
	}
}

func TestDaemonEnableAcceptsOneArg(t *testing.T) {
	svc := &stubDaemonService{}
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, svc)
	cmd := runner.newDaemonEnableCommand()
	err := cmd.Args(cmd, []string{"/some/path"})
	if err != nil {
		t.Fatalf("unexpected error for one arg: %v", err)
	}
}

func TestDaemonEnableDelegatesToService(t *testing.T) {
	svc := &stubDaemonService{}
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, svc)
	cmd := runner.newDaemonEnableCommand()
	cmd.SetArgs([]string{"/project"})
	if err := cmd.RunE(cmd, []string{"/project"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.lastEnable != "/project" {
		t.Fatalf("expected enable called with /project, got %q", svc.lastEnable)
	}
}

func TestDaemonEnablePropagatesServiceError(t *testing.T) {
	svc := &stubDaemonService{enableErr: errors.New("enable failed")}
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, svc)
	cmd := runner.newDaemonEnableCommand()
	err := cmd.RunE(cmd, []string{"/project"})
	if err == nil {
		t.Fatal("expected error from service to propagate")
	}
}

// ---- newDaemonDisableCommand ----

func TestDaemonDisableRequiresOneArg(t *testing.T) {
	svc := &stubDaemonService{}
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, svc)
	cmd := runner.newDaemonDisableCommand()
	err := cmd.Args(cmd, []string{})
	if err == nil {
		t.Fatal("expected error when disable called with no args")
	}
}

func TestDaemonDisableRejectsMultipleArgs(t *testing.T) {
	svc := &stubDaemonService{}
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, svc)
	cmd := runner.newDaemonDisableCommand()
	err := cmd.Args(cmd, []string{"a", "b"})
	if err == nil {
		t.Fatal("expected error when disable called with two args")
	}
}

func TestDaemonDisableDelegatesToService(t *testing.T) {
	svc := &stubDaemonService{}
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, svc)
	cmd := runner.newDaemonDisableCommand()
	if err := cmd.RunE(cmd, []string{"/project"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.lastDisable != "/project" {
		t.Fatalf("expected disable called with /project, got %q", svc.lastDisable)
	}
}

func TestDaemonDisablePropagatesServiceError(t *testing.T) {
	svc := &stubDaemonService{disableErr: errors.New("disable failed")}
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, svc)
	cmd := runner.newDaemonDisableCommand()
	err := cmd.RunE(cmd, []string{"/project"})
	if err == nil {
		t.Fatal("expected error from service to propagate")
	}
}

// ---- newDaemonStatusCommand ----

func TestDaemonStatusDelegatesToService(t *testing.T) {
	svc := &stubDaemonService{}
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, svc)
	cmd := runner.newDaemonStatusCommand()
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDaemonStatusPropagatesServiceError(t *testing.T) {
	svc := &stubDaemonService{statusErr: errors.New("status failed")}
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, svc)
	cmd := runner.newDaemonStatusCommand()
	err := cmd.RunE(cmd, []string{})
	if err == nil {
		t.Fatal("expected error from service to propagate")
	}
}
