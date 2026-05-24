package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// ---- renderSkillsHelp ----

func TestRenderSkillsHelpContainsInstallCommand(t *testing.T) {
	got := renderSkillsHelp()
	if !strings.Contains(got, "install") {
		t.Fatalf("expected 'install' in skills help, got %q", got)
	}
}

func TestRenderSkillsHelpContainsSupportedEditors(t *testing.T) {
	got := renderSkillsHelp()
	for _, e := range skillsEditors {
		if !strings.Contains(got, e.id) {
			t.Fatalf("expected editor %q in skills help output", e.id)
		}
	}
}

func TestRenderSkillsHelpIsNonEmpty(t *testing.T) {
	if renderSkillsHelp() == "" {
		t.Fatal("expected non-empty skills help")
	}
}

// ---- renderSkillsInstallHelp ----

func TestRenderSkillsInstallHelpContainsEditors(t *testing.T) {
	got := renderSkillsInstallHelp()
	for _, e := range skillsEditors {
		if !strings.Contains(got, e.id) {
			t.Fatalf("expected editor %q in install help output", e.id)
		}
	}
}

func TestRenderSkillsInstallHelpMentionsGit(t *testing.T) {
	got := renderSkillsInstallHelp()
	if !strings.Contains(got, "git") {
		t.Fatalf("expected 'git' in install help, got %q", got)
	}
}

// ---- writeSkillsMissingEditorError ----

func TestWriteSkillsMissingEditorErrorWritesToCmdOutput(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, nil)
	cmd := runner.newSkillsInstallCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	writeSkillsMissingEditorError(cmd)
	if buf.Len() == 0 {
		t.Fatal("expected output written to cmd stdout")
	}
	if !strings.Contains(buf.String(), "Missing editor") {
		t.Fatalf("expected missing-editor message, got %q", buf.String())
	}
}

func TestWriteSkillsMissingEditorErrorListsAllEditors(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, nil)
	cmd := runner.newSkillsInstallCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	writeSkillsMissingEditorError(cmd)
	for _, e := range skillsEditors {
		if !strings.Contains(buf.String(), e.id) {
			t.Fatalf("expected editor %q in missing-editor output", e.id)
		}
	}
}

// ---- newSkillsInstallCommand ----

func TestSkillsInstallNoArgsPrintsErrorAndReturns(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, nil).
		WithSkillsCommand(stubSkillsCommand{})
	cmd := runner.newSkillsInstallCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	err := cmd.RunE(cmd, []string{})
	// Returns a non-nil error (empty string sentinel) to signal failure
	if err == nil {
		t.Fatal("expected non-nil error when no editor arg provided")
	}
}

func TestSkillsInstallDelegatesToService(t *testing.T) {
	stub := &captureSkillsCommand{}
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, nil).
		WithSkillsCommand(stub)
	cmd := runner.newSkillsInstallCommand()
	if err := cmd.RunE(cmd, []string{"claude"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stub.lastEditor != "claude" {
		t.Fatalf("expected Install called with 'claude', got %q", stub.lastEditor)
	}
}

func TestSkillsInstallPropagatesServiceError(t *testing.T) {
	stub := &errSkillsCommand{err: errors.New("install failed")}
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, nil).
		WithSkillsCommand(stub)
	cmd := runner.newSkillsInstallCommand()
	if err := cmd.RunE(cmd, []string{"copilot"}); err == nil {
		t.Fatal("expected error to propagate from Install")
	}
}

func TestSkillsInstallHasVerboseFlag(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, nil)
	cmd := runner.newSkillsInstallCommand()
	if cmd.Flags().Lookup("verbose") == nil {
		t.Fatal("expected --verbose flag to be registered")
	}
}

// ---- newSkillsCommand ----

func TestNewSkillsCommandHasInstallSubcommand(t *testing.T) {
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil, nil)
	cmd := runner.newSkillsCommand()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "install" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected 'install' subcommand under 'skills'")
	}
}

type captureSkillsCommand struct {
	lastEditor  string
	lastVerbose bool
}

func (c *captureSkillsCommand) Install(editor string, verbose bool) error {
	c.lastEditor = editor
	c.lastVerbose = verbose
	return nil
}

type errSkillsCommand struct{ err error }

func (e *errSkillsCommand) Install(_ string, _ bool) error { return e.err }
