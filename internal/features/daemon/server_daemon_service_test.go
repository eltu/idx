package daemon_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"idx/internal/features/daemon"
	"idx/internal/features/daemon/mocks"
	sharedoutput "idx/internal/shared/output"
)

// makeInitializedProject creates the .idx/index.idx sentinel so Start does not reject the dir.
func makeInitializedProject(t *testing.T, dir string) {
	t.Helper()
	idxDir := filepath.Join(dir, ".idx")
	require.NoError(t, os.MkdirAll(idxDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(idxDir, "index.idx"), []byte{}, 0o600))
}

// newTestService builds a service with fully controllable socket and process checks.
// socketAlive is called for all isSocketAlive checks; use a stateful closure to
// return false on the first call (liveness check) and true after (readiness poll).
func newTestService(
	t *testing.T,
	repo *mocks.MockStateRepository,
	spawner *mocks.MockServerSpawner,
	processAlive func(int) bool,
	socketAlive func(string) bool,
) (*daemon.ServerDaemonService, *strings.Builder) {
	t.Helper()
	buf := &strings.Builder{}
	writer := sharedoutput.NewLineWriter(buf)
	svc := daemon.NewServerDaemonServiceWithDeps(daemon.ServerDaemonServiceDeps{
		StateRepo:        repo,
		Spawner:          spawner,
		Output:           writer,
		ProcessExists:    processAlive,
		IsSocketAlive:    socketAlive,
		ReadinessTimeout: 500 * time.Millisecond,
	})
	return svc, buf
}

// socketAliveAfterSpawn returns false on the first call (server not yet up),
// then true on all subsequent calls (readiness poll succeeds).
func socketAliveAfterSpawn() func(string) bool {
	calls := 0
	return func(_ string) bool {
		calls++
		return calls > 1
	}
}

func TestServerDaemonService_Start_SpawnsAndSavesState(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockStateRepository(ctrl)
	spawner := mocks.NewMockServerSpawner(ctrl)

	projectPath := t.TempDir()
	makeInitializedProject(t, projectPath)
	repo.EXPECT().ReadState(projectPath).Return(nil, nil)
	spawner.EXPECT().SpawnServerProcess(projectPath).Return(1234, nil)
	repo.EXPECT().SaveState(projectPath, gomock.Any()).Return(nil)

	svc, buf := newTestService(t, repo, spawner, func(_ int) bool { return false }, socketAliveAfterSpawn())
	err := svc.Start(projectPath)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "1234")
}

func TestServerDaemonService_Start_IdempotentWhenSocketAlive(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockStateRepository(ctrl)
	spawner := mocks.NewMockServerSpawner(ctrl)

	projectPath := t.TempDir()
	makeInitializedProject(t, projectPath)

	svc, buf := newTestService(t, repo, spawner,
		func(_ int) bool { return true },
		func(_ string) bool { return true }, // socket is alive from the first check
	)
	err := svc.Start(projectPath)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "already running")
}

func TestServerDaemonService_Start_KillsStaleProcessBeforeSpawning(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockStateRepository(ctrl)
	spawner := mocks.NewMockServerSpawner(ctrl)

	projectPath := t.TempDir()
	makeInitializedProject(t, projectPath)

	// State file has a live PID, but socket is dead (stale process)
	staleState := &daemon.ServerState{PID: 1, ProjectPath: projectPath}
	repo.EXPECT().ReadState(projectPath).Return(staleState, nil)
	repo.EXPECT().RemoveState(projectPath).Return(nil)
	spawner.EXPECT().SpawnServerProcess(projectPath).Return(1234, nil)
	repo.EXPECT().SaveState(projectPath, gomock.Any()).Return(nil)

	svc, buf := newTestService(t, repo, spawner,
		func(_ int) bool { return true }, // stale PID appears alive via kill -0
		socketAliveAfterSpawn(),          // socket dead initially, ready after spawn
	)
	err := svc.Start(projectPath)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "1234")
}

func TestServerDaemonService_Start_ErrorsWhenSocketNeverReady(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockStateRepository(ctrl)
	spawner := mocks.NewMockServerSpawner(ctrl)

	projectPath := t.TempDir()
	makeInitializedProject(t, projectPath)
	repo.EXPECT().ReadState(projectPath).Return(nil, nil)
	spawner.EXPECT().SpawnServerProcess(projectPath).Return(1234, nil)

	svc, _ := newTestService(t, repo, spawner,
		func(_ int) bool { return false },
		func(_ string) bool { return false }, // socket never becomes ready
	)
	err := svc.Start(projectPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "did not become ready")
}

func TestServerDaemonService_Start_KillsOrphanOnStateSaveFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockStateRepository(ctrl)
	spawner := mocks.NewMockServerSpawner(ctrl)

	projectPath := t.TempDir()
	makeInitializedProject(t, projectPath)
	repo.EXPECT().ReadState(projectPath).Return(nil, nil)
	spawner.EXPECT().SpawnServerProcess(projectPath).Return(1, nil) // PID 1 is signal-safe
	repo.EXPECT().SaveState(projectPath, gomock.Any()).Return(assert.AnError)

	svc, _ := newTestService(t, repo, spawner,
		func(_ int) bool { return false },
		socketAliveAfterSpawn(),
	)
	err := svc.Start(projectPath)
	require.Error(t, err)
}

func TestServerDaemonService_Start_ReturnsErrNotInitializedWhenNoIndex(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockStateRepository(ctrl)
	spawner := mocks.NewMockServerSpawner(ctrl)

	projectPath := t.TempDir() // no .idx/index.idx

	svc, _ := newTestService(t, repo, spawner,
		func(_ int) bool { return false },
		func(_ string) bool { return false },
	)
	err := svc.Start(projectPath)
	require.Error(t, err)
	assert.ErrorIs(t, err, daemon.ErrNotInitialized)
}

func TestServerDaemonService_Stop_SendsSIGTERMAndRemovesState(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockStateRepository(ctrl)
	spawner := mocks.NewMockServerSpawner(ctrl)

	projectPath := t.TempDir()
	state := &daemon.ServerState{PID: 1, ProjectPath: projectPath}
	repo.EXPECT().ReadState(projectPath).Return(state, nil)
	repo.EXPECT().RemoveState(projectPath).Return(nil)

	svc, buf := newTestService(t, repo, spawner,
		func(_ int) bool { return true },
		func(_ string) bool { return true }, // socket alive → stop sends SIGTERM
	)
	err := svc.Stop(projectPath)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "stopped")
}

func TestServerDaemonService_Stop_NoopWhenSocketDead(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockStateRepository(ctrl)
	spawner := mocks.NewMockServerSpawner(ctrl)

	projectPath := t.TempDir()
	repo.EXPECT().ReadState(projectPath).Return(nil, nil)

	svc, buf := newTestService(t, repo, spawner,
		func(_ int) bool { return false },
		func(_ string) bool { return false }, // socket dead
	)
	err := svc.Stop(projectPath)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "not running")
}

func TestServerDaemonService_Status_ShowsRunningDetails(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockStateRepository(ctrl)
	spawner := mocks.NewMockServerSpawner(ctrl)

	projectPath := t.TempDir()
	state := &daemon.ServerState{
		PID:        5678,
		StartedAt:  time.Now().Add(-30 * time.Second),
		SocketPath: "/tmp/test.sock",
	}
	repo.EXPECT().ReadState(projectPath).Return(state, nil)

	svc, buf := newTestService(t, repo, spawner,
		func(_ int) bool { return true },
		func(_ string) bool { return true },
	)
	err := svc.Status(projectPath)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "5678")
}

func TestServerDaemonService_Status_ShowsNotRunning(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockStateRepository(ctrl)
	spawner := mocks.NewMockServerSpawner(ctrl)

	projectPath := t.TempDir()
	repo.EXPECT().ReadState(projectPath).Return(nil, nil)

	svc, buf := newTestService(t, repo, spawner,
		func(_ int) bool { return false },
		func(_ string) bool { return false },
	)
	err := svc.Status(projectPath)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "not running")
}

func TestServerDaemonService_IsProjectMonitored_TrueWhenSocketAlive(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockStateRepository(ctrl)
	spawner := mocks.NewMockServerSpawner(ctrl)

	projectPath := t.TempDir()

	svc, _ := newTestService(t, repo, spawner,
		func(_ int) bool { return true },
		func(_ string) bool { return true },
	)
	monitored, err := svc.IsProjectMonitored(projectPath)
	require.NoError(t, err)
	assert.True(t, monitored)
}

func TestServerDaemonService_IsProjectMonitored_FalseWhenSocketDead(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockStateRepository(ctrl)
	spawner := mocks.NewMockServerSpawner(ctrl)

	projectPath := t.TempDir()

	svc, _ := newTestService(t, repo, spawner,
		func(_ int) bool { return false },
		func(_ string) bool { return false },
	)
	monitored, err := svc.IsProjectMonitored(projectPath)
	require.NoError(t, err)
	assert.False(t, monitored)
}

func TestServerDaemonService_ProjectStatus_ReturnsDetailsWhenRunning(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockStateRepository(ctrl)
	spawner := mocks.NewMockServerSpawner(ctrl)

	projectPath := t.TempDir()
	startedAt := time.Now().Add(-time.Minute)
	state := &daemon.ServerState{PID: 42, StartedAt: startedAt}
	repo.EXPECT().ReadState(projectPath).Return(state, nil)

	svc, _ := newTestService(t, repo, spawner,
		func(_ int) bool { return true },
		func(_ string) bool { return true },
	)
	status, err := svc.ProjectStatus(projectPath)
	require.NoError(t, err)
	require.NotNil(t, status)
	assert.True(t, status.Enabled)
	assert.Equal(t, 42, status.PID)
	assert.Equal(t, startedAt.Unix(), status.StartedAt.Unix())
}
