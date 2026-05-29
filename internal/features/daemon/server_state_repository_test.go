package daemon_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"idx/internal/features/daemon"
)

func TestServerStateRepository_ReadState_ReturnsNilWhenMissing(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := daemon.NewServerStateRepository()

	// Act
	state, err := repo.ReadState("/nonexistent/project/path")

	// Assert
	require.NoError(t, err)
	assert.Nil(t, state)
}

func TestServerStateRepository_SaveAndRead_RoundTripsState(t *testing.T) {
	t.Parallel()

	// Arrange
	projectPath := t.TempDir()
	repo := daemon.NewServerStateRepository()
	want := &daemon.ServerState{
		PID:         42,
		StartedAt:   time.Now().Truncate(time.Second),
		SocketPath:  "/tmp/test.sock",
		ProjectPath: projectPath,
	}

	// Act
	require.NoError(t, repo.SaveState(projectPath, want))
	got, err := repo.ReadState(projectPath)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, want.PID, got.PID)
	assert.Equal(t, want.SocketPath, got.SocketPath)
	assert.Equal(t, want.ProjectPath, got.ProjectPath)
	assert.Equal(t, want.StartedAt.Unix(), got.StartedAt.Unix())
}

func TestServerStateRepository_RemoveState_DeletesPersistedState(t *testing.T) {
	t.Parallel()

	// Arrange
	projectPath := t.TempDir()
	repo := daemon.NewServerStateRepository()
	state := &daemon.ServerState{PID: 99, ProjectPath: projectPath}
	require.NoError(t, repo.SaveState(projectPath, state))

	// Act
	require.NoError(t, repo.RemoveState(projectPath))
	got, err := repo.ReadState(projectPath)

	// Assert
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestServerStateRepository_RemoveState_IdempotentWhenMissing(t *testing.T) {
	t.Parallel()

	// Arrange
	repo := daemon.NewServerStateRepository()

	// Act & Assert
	assert.NoError(t, repo.RemoveState("/nonexistent/project/path"))
}

func TestServerStateRepository_SaveState_WritesFileInsideProject(t *testing.T) {
	t.Parallel()

	// Arrange
	projectPath := t.TempDir()
	repo := daemon.NewServerStateRepository()
	state := &daemon.ServerState{PID: 1}

	// Act
	require.NoError(t, repo.SaveState(projectPath, state))

	// Assert
	stateFile := filepath.Join(projectPath, ".idx", "server.state")
	_, err := os.Stat(stateFile)
	assert.NoError(t, err, "expected state file at %s", stateFile)
}
