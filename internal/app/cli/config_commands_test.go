package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- newConfigGetCommand ----

func TestNewConfigGetCommand_RegisteredAsSubcommand(t *testing.T) {
	t.Parallel()

	// Arrange
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	configCmd := runner.newConfigCommand()

	// Act
	var found bool
	for _, sub := range configCmd.Commands() {
		if sub.Use == "get <key>" {
			found = true
			break
		}
	}

	// Assert
	assert.True(t, found, "expected 'get' to be registered as a subcommand of config")
}

// ---- runConfigGetTo ----

func TestRunConfigGetTo_ValidKey_PrintsValue(t *testing.T) {
	t.Parallel()

	// Arrange
	var buf bytes.Buffer
	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)

	// Act
	err := runner.runConfigGetTo(&buf, "search.operator")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "AND", strings.TrimSpace(buf.String()))
}

func TestRunConfigGetTo_UnknownKey_ReturnsError(t *testing.T) {
	t.Parallel()

	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	err := runner.runConfigGetTo(&bytes.Buffer{}, "search.nonexistent")
	require.Error(t, err)
	assert.ErrorContains(t, err, "unknown config key")
}

func TestRunConfigGetTo_ErrorMessage_ListsValidKeys(t *testing.T) {
	t.Parallel()

	runner := NewCommandRunner([]string{"idx"}, nil, nil, nil)
	err := runner.runConfigGetTo(&bytes.Buffer{}, "unknown.key")
	require.Error(t, err)
	// The error message should list at least one known key so the user can self-correct.
	assert.ErrorContains(t, err, "search.operator")
}
