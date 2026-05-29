package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- renderSkillsHelp ----

func TestRenderSkillsHelp_ContainsInstallCommand(t *testing.T) {
	t.Parallel()
	assert.Contains(t, renderSkillsHelp(), "install")
}

func TestRenderSkillsHelp_ContainsSupportedEditors(t *testing.T) {
	t.Parallel()
	got := renderSkillsHelp()
	for _, e := range skillsEditors {
		assert.Contains(t, got, e.id, "expected editor %q in skills help output", e.id)
	}
}

func TestRenderSkillsHelp_IsNonEmpty(t *testing.T) {
	t.Parallel()
	assert.NotEmpty(t, renderSkillsHelp())
}

// ---- renderSkillsInstallHelp ----

func TestRenderSkillsInstallHelp_ContainsAllEditors(t *testing.T) {
	t.Parallel()
	got := renderSkillsInstallHelp()
	for _, e := range skillsEditors {
		assert.Contains(t, got, e.id, "expected editor %q in install help output", e.id)
	}
}

func TestRenderSkillsInstallHelp_MentionsBundled(t *testing.T) {
	t.Parallel()
	assert.Contains(t, renderSkillsInstallHelp(), "bundled")
}

// ---- writeSkillsMissingEditorError ----

func TestWriteSkillsMissingEditorError_WritesMessageWithAllEditors(t *testing.T) {
	t.Parallel()

	// Arrange
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	cmd := runner.newSkillsInstallCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)

	// Act
	writeSkillsMissingEditorError(cmd)

	// Assert
	require.NotEmpty(t, buf.String())
	assert.True(t, strings.Contains(buf.String(), "Missing editor"), "expected missing-editor message")
	for _, e := range skillsEditors {
		assert.Contains(t, buf.String(), e.id, "expected editor %q in missing-editor output", e.id)
	}
}

// ---- newSkillsInstallCommand ----

func TestSkillsInstallCommand_NoArgs_ReturnsError(t *testing.T) {
	t.Parallel()
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).
		WithSkillsCommand(stubSkillsCommand{})
	cmd := runner.newSkillsInstallCommand()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	// Returns a non-nil error (empty string sentinel) to signal failure
	require.Error(t, cmd.RunE(cmd, []string{}))
}

func TestSkillsInstallCommand_ValidEditor_DelegatesToService(t *testing.T) {
	t.Parallel()

	// Arrange
	stub := &captureSkillsCommand{}
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).
		WithSkillsCommand(stub)
	cmd := runner.newSkillsInstallCommand()

	// Act
	err := cmd.RunE(cmd, []string{"claude"})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "claude", stub.lastEditor)
}

func TestSkillsInstallCommand_ServiceError_Propagates(t *testing.T) {
	t.Parallel()
	stub := &errSkillsCommand{err: errors.New("install failed")}
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil).
		WithSkillsCommand(stub)
	cmd := runner.newSkillsInstallCommand()
	require.Error(t, cmd.RunE(cmd, []string{"copilot"}))
}

func TestSkillsInstallCommand_HasNoVerboseFlag(t *testing.T) {
	t.Parallel()
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	cmd := runner.newSkillsInstallCommand()
	assert.Nil(t, cmd.Flags().Lookup("verbose"), "expected --verbose flag to be absent")
}

// ---- newSkillsCommand ----

func TestNewSkillsCommand_HasInstallSubcommand(t *testing.T) {
	t.Parallel()
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	cmd := runner.newSkillsCommand()
	found := false
	for _, sub := range cmd.Commands() {
		if sub.Name() == "install" {
			found = true
		}
	}
	assert.True(t, found, "expected 'install' subcommand under 'skills'")
}

type captureSkillsCommand struct {
	lastEditor string
}

func (c *captureSkillsCommand) Install(editor string) error {
	c.lastEditor = editor
	return nil
}

type errSkillsCommand struct{ err error }

func (e *errSkillsCommand) Install(_ string) error { return e.err }
