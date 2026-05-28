package daemon_test

import (
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

func newTestServerService(
	t *testing.T,
	repo *mocks.MockStateRepository,
	spawner *mocks.MockServerSpawner,
	alive func(int) bool,
) (*daemon.ServerDaemonService, *strings.Builder) {
	t.Helper()
	buf := &strings.Builder{}
	writer := sharedoutput.NewLineWriter(buf)
	svc := daemon.NewServerDaemonServiceWithProcessChecker(repo, spawner, writer, alive)
	return svc, buf
}

func TestServerDaemonService_Start_SpawnsAndSavesState(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockStateRepository(ctrl)
	spawner := mocks.NewMockServerSpawner(ctrl)

	projectPath := t.TempDir()
	repo.EXPECT().ReadState(projectPath).Return(nil, nil)
	spawner.EXPECT().SpawnServerProcess(projectPath).Return(1234, nil)
	repo.EXPECT().SaveState(projectPath, gomock.Any()).Return(nil)

	svc, buf := newTestServerService(t, repo, spawner, func(_ int) bool { return false })
	err := svc.Start(projectPath)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "1234")
}

func TestServerDaemonService_Start_IdempotentWhenAlreadyRunning(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockStateRepository(ctrl)
	spawner := mocks.NewMockServerSpawner(ctrl)

	projectPath := t.TempDir()
	state := &daemon.ServerState{PID: 999, ProjectPath: projectPath}
	repo.EXPECT().ReadState(projectPath).Return(state, nil)

	svc, buf := newTestServerService(t, repo, spawner, func(_ int) bool { return true })
	err := svc.Start(projectPath)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "already running")
}

func TestServerDaemonService_Start_KillsOrphanOnStateSaveFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockStateRepository(ctrl)
	spawner := mocks.NewMockServerSpawner(ctrl)

	projectPath := t.TempDir()
	repo.EXPECT().ReadState(projectPath).Return(nil, nil)
	spawner.EXPECT().SpawnServerProcess(projectPath).Return(1, nil) // PID 1 is safe to signal(0)
	repo.EXPECT().SaveState(projectPath, gomock.Any()).Return(assert.AnError)

	svc, _ := newTestServerService(t, repo, spawner, func(_ int) bool { return false })
	err := svc.Start(projectPath)
	require.Error(t, err)
}

func TestServerDaemonService_Stop_SendsSIGTERMAndRemovesState(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockStateRepository(ctrl)
	spawner := mocks.NewMockServerSpawner(ctrl)

	projectPath := t.TempDir()
	state := &daemon.ServerState{PID: 1, ProjectPath: projectPath}
	repo.EXPECT().ReadState(projectPath).Return(state, nil)
	repo.EXPECT().RemoveState(projectPath).Return(nil)

	svc, buf := newTestServerService(t, repo, spawner, func(_ int) bool { return true })
	err := svc.Stop(projectPath)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "stopped")
}

func TestServerDaemonService_Stop_NoopWhenNotRunning(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockStateRepository(ctrl)
	spawner := mocks.NewMockServerSpawner(ctrl)

	projectPath := t.TempDir()
	repo.EXPECT().ReadState(projectPath).Return(nil, nil)

	svc, buf := newTestServerService(t, repo, spawner, func(_ int) bool { return false })
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

	svc, buf := newTestServerService(t, repo, spawner, func(_ int) bool { return true })
	err := svc.Status(projectPath)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "5678")
	assert.Contains(t, buf.String(), "/tmp/test.sock")
}

func TestServerDaemonService_Status_ShowsNotRunning(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockStateRepository(ctrl)
	spawner := mocks.NewMockServerSpawner(ctrl)

	projectPath := t.TempDir()
	repo.EXPECT().ReadState(projectPath).Return(nil, nil)

	svc, buf := newTestServerService(t, repo, spawner, func(_ int) bool { return false })
	err := svc.Status(projectPath)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "not running")
}

func TestServerDaemonService_IsProjectMonitored_TrueWhenRunning(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockStateRepository(ctrl)
	spawner := mocks.NewMockServerSpawner(ctrl)

	projectPath := t.TempDir()
	state := &daemon.ServerState{PID: 777}
	repo.EXPECT().ReadState(projectPath).Return(state, nil)

	svc, _ := newTestServerService(t, repo, spawner, func(_ int) bool { return true })
	monitored, err := svc.IsProjectMonitored(projectPath)
	require.NoError(t, err)
	assert.True(t, monitored)
}

func TestServerDaemonService_IsProjectMonitored_FalseWhenStateMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	repo := mocks.NewMockStateRepository(ctrl)
	spawner := mocks.NewMockServerSpawner(ctrl)

	projectPath := t.TempDir()
	repo.EXPECT().ReadState(projectPath).Return(nil, nil)

	svc, _ := newTestServerService(t, repo, spawner, func(_ int) bool { return false })
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

	svc, _ := newTestServerService(t, repo, spawner, func(_ int) bool { return true })
	status, err := svc.ProjectStatus(projectPath)
	require.NoError(t, err)
	require.NotNil(t, status)
	assert.True(t, status.Enabled)
	assert.Equal(t, 42, status.PID)
	assert.Equal(t, startedAt.Unix(), status.StartedAt.Unix())
}
