package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

type stubSkillsCommand struct{ err error }

func (s stubSkillsCommand) Install(_ string, _ bool) error { return s.err }

func TestRenderSkillsHelpContainsExpectedSections(t *testing.T) {
	help := renderSkillsHelp()
	for _, expected := range []string{"idx skills", "Usage", "Commands", "Editors", "Examples"} {
		if !strings.Contains(help, expected) {
			t.Fatalf("expected %q in skills help output", expected)
		}
	}
}

func TestRenderSkillsHelpListsAllEditors(t *testing.T) {
	help := renderSkillsHelp()
	for _, e := range skillsEditors {
		if !strings.Contains(help, e.id) {
			t.Fatalf("expected editor %q in skills help", e.id)
		}
	}
}

func TestRenderSkillsInstallHelpContainsExpectedSections(t *testing.T) {
	help := renderSkillsInstallHelp()
	for _, expected := range []string{"idx skills install", "Usage", "Editors", "Examples", "git"} {
		if !strings.Contains(help, expected) {
			t.Fatalf("expected %q in skills install help output", expected)
		}
	}
}

func TestSkillsInstallCommandReturnsErrorWhenNoEditorGiven(t *testing.T) {
	runner := NewCommandRunner(
		[]string{"idx", "skills", "install"},
		noOpIndexCommand{},
		noOpDestroyCommand{},
		noOpSearchCommand{},
		noOpDaemonCommand{},
	)
	runner.skillsCommand = stubSkillsCommand{}

	cmd := runner.newSkillsInstallCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.RunE(cmd, []string{})
	if err == nil {
		t.Fatal("expected error when no editor argument is provided, got nil")
	}
}

func TestSkillsInstallCommandDelegatesToInstaller(t *testing.T) {
	runner := NewCommandRunner(
		[]string{"idx", "skills", "install", "claude"},
		noOpIndexCommand{},
		noOpDestroyCommand{},
		noOpSearchCommand{},
		noOpDaemonCommand{},
	)
	runner.skillsCommand = stubSkillsCommand{err: errors.New("install failed")}

	cmd := runner.newSkillsInstallCommand()
	err := cmd.RunE(cmd, []string{"claude"})
	if err == nil {
		t.Fatal("expected error from installer, got nil")
	}
}

func TestSkillsInstallCommandSucceeds(t *testing.T) {
	runner := NewCommandRunner(
		[]string{"idx", "skills", "install", "claude"},
		noOpIndexCommand{},
		noOpDestroyCommand{},
		noOpSearchCommand{},
		noOpDaemonCommand{},
	)
	runner.skillsCommand = stubSkillsCommand{}

	cmd := runner.newSkillsInstallCommand()
	if err := cmd.RunE(cmd, []string{"claude"}); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}
