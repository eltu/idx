package daemon_test

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"idx/internal/features/daemon"
)

func TestOSServerSpawner_SpawnServerProcess_SetsDaemonEnvAndDir(t *testing.T) {
	projectPath := t.TempDir()

	var capturedCmd *exec.Cmd
	spawner := daemon.NewOSServerSpawnerWithDeps(
		func() (string, error) { return "/usr/bin/idx", nil },
		func(name string, args ...string) *exec.Cmd {
			// Use 'true' to avoid actually starting idx
			cmd := exec.Command("true")
			capturedCmd = cmd
			capturedCmd.Args = append([]string{name}, args...)
			return capturedCmd
		},
	)

	pid, err := spawner.SpawnServerProcess(projectPath)
	require.NoError(t, err)
	assert.Positive(t, pid)

	assert.Equal(t, []string{"/usr/bin/idx", "server", "run"}, capturedCmd.Args)
	assert.Equal(t, projectPath, capturedCmd.Dir)

	var hasDaemonEnv bool
	for _, env := range capturedCmd.Env {
		if env == "IDX_SERVER_DAEMON=1" {
			hasDaemonEnv = true
		}
	}
	assert.True(t, hasDaemonEnv, "expected IDX_SERVER_DAEMON=1 in environment")
}

func TestOSServerSpawner_SpawnServerProcess_ErrorOnBadExecutable(t *testing.T) {
	spawner := daemon.NewOSServerSpawnerWithDeps(
		func() (string, error) { return "", assert.AnError },
		exec.Command,
	)

	_, err := spawner.SpawnServerProcess(t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve executable path")
}
