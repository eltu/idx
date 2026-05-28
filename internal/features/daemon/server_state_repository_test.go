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
	repo := daemon.NewServerStateRepository()
	state, err := repo.ReadState("/nonexistent/project/path")
	require.NoError(t, err)
	assert.Nil(t, state)
}

func TestServerStateRepository_SaveAndRead(t *testing.T) {
	projectPath := t.TempDir()
	repo := daemon.NewServerStateRepository()

	want := &daemon.ServerState{
		PID:         42,
		StartedAt:   time.Now().Truncate(time.Second),
		SocketPath:  "/tmp/test.sock",
		ProjectPath: projectPath,
	}

	require.NoError(t, repo.SaveState(projectPath, want))

	got, err := repo.ReadState(projectPath)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, want.PID, got.PID)
	assert.Equal(t, want.SocketPath, got.SocketPath)
	assert.Equal(t, want.ProjectPath, got.ProjectPath)
	assert.Equal(t, want.StartedAt.Unix(), got.StartedAt.Unix())
}

func TestServerStateRepository_RemoveState(t *testing.T) {
	projectPath := t.TempDir()
	repo := daemon.NewServerStateRepository()

	state := &daemon.ServerState{PID: 99, ProjectPath: projectPath}
	require.NoError(t, repo.SaveState(projectPath, state))

	require.NoError(t, repo.RemoveState(projectPath))

	got, err := repo.ReadState(projectPath)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestServerStateRepository_RemoveState_IdempotentWhenMissing(t *testing.T) {
	repo := daemon.NewServerStateRepository()
	err := repo.RemoveState("/nonexistent/project/path")
	assert.NoError(t, err)
}

func TestServerStateRepository_StateFilePath_UsesProjectBaseName(t *testing.T) {
	home, _ := os.UserHomeDir()
	projectPath := t.TempDir()
	baseName := filepath.Base(projectPath)

	repo := daemon.NewServerStateRepository()
	state := &daemon.ServerState{PID: 1}
	require.NoError(t, repo.SaveState(projectPath, state))

	stateFile := filepath.Join(home, ".idx", baseName+".server.state")
	_, err := os.Stat(stateFile)
	assert.NoError(t, err, "expected state file at %s", stateFile)
}
