package daemon_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"idx/internal/core/domain"
	"idx/internal/core/services/daemon"
	"idx/internal/core/services/daemon/mocks"
)

type daemonTestEnv struct {
	t             *testing.T
	repo          *mocks.MockDaemonRepository
	tree          *mocks.MockProjectTree
	output        *mocks.MockTextOutput
	initCommand   *mocks.MockInitCommandInterface
	spawner       *mocks.MockProcessSpawner
	state         *domain.DaemonState
	lines         []string
	nextSpawnPID  int
	processExists func(int) bool
}

func newDaemonTestEnv(t *testing.T, initialState *domain.DaemonState) *daemonTestEnv {
	t.Helper()

	controller := gomock.NewController(t)
	t.Cleanup(controller.Finish)

	env := &daemonTestEnv{
		t:            t,
		repo:         mocks.NewMockDaemonRepository(controller),
		tree:         mocks.NewMockProjectTree(controller),
		output:       mocks.NewMockTextOutput(controller),
		initCommand:  mocks.NewMockInitCommandInterface(controller),
		spawner:      mocks.NewMockProcessSpawner(controller),
		state:        cloneDaemonState(initialState),
		nextSpawnPID: 12345,
		processExists: func(pid int) bool {
			return pid > 0
		},
	}

	env.repo.EXPECT().ReadState().DoAndReturn(func() (*domain.DaemonState, error) {
		return cloneDaemonState(env.state), nil
	}).AnyTimes()

	env.repo.EXPECT().SaveState(gomock.Any()).DoAndReturn(func(state *domain.DaemonState) error {
		env.state = cloneDaemonState(state)
		return nil
	}).AnyTimes()

	env.repo.EXPECT().UpdateProjectPID(gomock.Any(), gomock.Any()).DoAndReturn(func(projectPath string, pid int) error {
		if env.state == nil {
			return nil
		}

		for index, project := range env.state.Projects {
			if project.Path == projectPath {
				env.state.Projects[index].PID = pid
				break
			}
		}

		return nil
	}).AnyTimes()

	env.output.EXPECT().WriteLine(gomock.Any()).DoAndReturn(func(line string) error {
		env.lines = append(env.lines, line)
		return nil
	}).AnyTimes()

	return env
}

func (env *daemonTestEnv) service() *daemon.DaemonService {
	env.t.Helper()
	return daemon.NewDaemonServiceWithProcessChecker(env.repo, env.tree, env.output, env.initCommand, env.spawner, env.processExists)
}

func (env *daemonTestEnv) expectSpawn(projectPath string) int {
	env.t.Helper()

	pid := env.nextSpawnPID
	env.nextSpawnPID++
	env.spawner.EXPECT().SpawnWatchProcess(projectPath).Return(pid, nil).Times(1)
	return pid
}

func (env *daemonTestEnv) resetLines() {
	env.t.Helper()
	env.lines = nil
}

func cloneDaemonState(state *domain.DaemonState) *domain.DaemonState {
	if state == nil {
		return nil
	}

	clone := &domain.DaemonState{
		Projects:  make([]domain.MonitoredProject, len(state.Projects)),
		UpdatedAt: state.UpdatedAt,
	}

	copy(clone.Projects, state.Projects)
	return clone
}

func createIndexFile(t *testing.T, projectDir string) {
	t.Helper()

	indexDir := filepath.Join(projectDir, ".idx")
	if err := os.MkdirAll(indexDir, 0o750); err != nil {
		t.Fatalf("failed to create index directory: %v", err)
	}

	indexPath := filepath.Join(indexDir, "index.gob")
	indexPath = filepath.Join(indexDir, "index.idx")
	if err := os.WriteFile(indexPath, []byte("fake index"), 0o600); err != nil {
		t.Fatalf("failed to create index file: %v", err)
	}
}

func monitoredProject(path string, pid int) domain.MonitoredProject {
	return domain.MonitoredProject{
		Path:      path,
		PID:       pid,
		Enabled:   true,
		StartedAt: time.Now(),
		LastSync:  time.Now(),
	}
}
