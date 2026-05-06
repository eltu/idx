package daemon_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"

	"idx/internal/core/domain"
	"idx/internal/core/services/daemon"
	"idx/internal/core/services/daemon/mocks"
)

func TestDaemonServiceEnableCreatesNewProject(t *testing.T) {
	projectDir := t.TempDir()
	createIndexFile(t, projectDir)

	env := newDaemonTestEnv(t, nil)
	env.expectSpawn(projectDir)

	err := env.service().Enable(projectDir)
	if err != nil {
		t.Fatalf("expected enable to succeed, got %v", err)
	}

	if env.state == nil || len(env.state.Projects) != 1 {
		t.Fatalf("expected 1 project, got %#v", env.state)
	}
	if env.state.Projects[0].Path != projectDir {
		t.Fatalf("expected path %s, got %s", projectDir, env.state.Projects[0].Path)
	}
	if !env.state.Projects[0].Enabled {
		t.Fatal("expected project to be enabled")
	}
}

func TestDaemonServiceEnableAutoInitsWhenIndexMissing(t *testing.T) {
	projectDir := t.TempDir()

	env := newDaemonTestEnv(t, nil)
	env.initCommand.EXPECT().RunFromPath(projectDir).Return(nil).Times(1)
	env.expectSpawn(projectDir)

	err := env.service().Enable(projectDir)
	if err != nil {
		t.Fatalf("expected enable to succeed, got %v", err)
	}

	if len(env.lines) == 0 || env.lines[0] != "ℹ️  Index not found. Creating initial index..." {
		t.Fatalf("expected init message, got %#v", env.lines)
	}
}

func TestDaemonServiceEnableReturnsErrorWhenPathDoesNotExist(t *testing.T) {
	env := newDaemonTestEnv(t, nil)

	err := env.service().Enable(filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Fatal("expected error for nonexistent path, got nil")
	}
}

func TestDaemonServiceEnableReturnsErrorWhenProjectAlreadyMonitored(t *testing.T) {
	projectDir := t.TempDir()
	initialState := &domain.DaemonState{
		Projects: []domain.MonitoredProject{monitoredProject(projectDir, 1234)},
	}

	env := newDaemonTestEnv(t, initialState)

	err := env.service().Enable(projectDir)
	if err == nil {
		t.Fatal("expected error for already monitored project, got nil")
	}
}

func TestDaemonServiceEnableReturnsErrorWhenInitFails(t *testing.T) {
	projectDir := t.TempDir()
	env := newDaemonTestEnv(t, nil)
	env.initCommand.EXPECT().RunFromPath(projectDir).Return(errors.New("init failed")).Times(1)

	err := env.service().Enable(projectDir)
	if err == nil {
		t.Fatal("expected error when init fails, got nil")
	}
}

func TestDaemonServiceDisableRemovesProject(t *testing.T) {
	projectDir := t.TempDir()
	initialState := &domain.DaemonState{
		Projects: []domain.MonitoredProject{monitoredProject(projectDir, 12345)},
	}

	env := newDaemonTestEnv(t, initialState)

	err := env.service().Disable(projectDir)
	if err != nil {
		t.Fatalf("expected disable to succeed, got %v", err)
	}

	if len(env.state.Projects) != 0 {
		t.Fatalf("expected 0 projects after disable, got %d", len(env.state.Projects))
	}
}

func TestDaemonServiceDisableReturnsErrorWhenProjectNotMonitored(t *testing.T) {
	env := newDaemonTestEnv(t, &domain.DaemonState{Projects: []domain.MonitoredProject{}})

	err := env.service().Disable(t.TempDir())
	if err == nil {
		t.Fatal("expected error when project not monitored, got nil")
	}
}

func TestDaemonServiceStatusReturnsEmptyWhenNoProjectsMonitored(t *testing.T) {
	env := newDaemonTestEnv(t, nil)

	err := env.service().Status()
	if err != nil {
		t.Fatalf("expected status to succeed, got %v", err)
	}

	if len(env.lines) != 1 || env.lines[0] != "❌ No projects being monitored" {
		t.Fatalf("unexpected status output: %#v", env.lines)
	}
}

func TestDaemonServiceStatusListsMonitoredProjects(t *testing.T) {
	projectDir := t.TempDir()
	env := newDaemonTestEnv(t, &domain.DaemonState{
		Projects: []domain.MonitoredProject{monitoredProject(projectDir, 12345)},
	})

	err := env.service().Status()
	if err != nil {
		t.Fatalf("expected status to succeed, got %v", err)
	}

	if len(env.lines) < 2 {
		t.Fatalf("expected header and one project line, got %#v", env.lines)
	}
	if env.lines[0] != "📊 Monitored Projects:" {
		t.Fatalf("expected monitored projects header, got %q", env.lines[0])
	}
	if !strings.Contains(env.lines[1], "✅ running") {
		t.Fatalf("expected running project line, got %q", env.lines[1])
	}
}

func TestDaemonServiceStatusMarksProjectStoppedWhenPIDIsNotAlive(t *testing.T) {
	projectDir := t.TempDir()
	env := newDaemonTestEnv(t, &domain.DaemonState{
		Projects: []domain.MonitoredProject{monitoredProject(projectDir, 12345)},
	})
	env.processExists = func(int) bool { return false }

	err := env.service().Status()
	if err != nil {
		t.Fatalf("expected status to succeed, got %v", err)
	}

	if len(env.lines) < 2 {
		t.Fatalf("expected header and one project line, got %#v", env.lines)
	}
	if !strings.Contains(env.lines[1], "❌ stopped") {
		t.Fatalf("expected stopped project line, got %q", env.lines[1])
	}
}

func TestDaemonServiceEnableMultipleProjects(t *testing.T) {
	projectDir1 := t.TempDir()
	projectDir2 := t.TempDir()
	createIndexFile(t, projectDir1)
	createIndexFile(t, projectDir2)

	env := newDaemonTestEnv(t, nil)
	env.expectSpawn(projectDir1)
	env.expectSpawn(projectDir2)

	service := env.service()
	if err := service.Enable(projectDir1); err != nil {
		t.Fatalf("expected first enable to succeed, got %v", err)
	}
	if err := service.Enable(projectDir2); err != nil {
		t.Fatalf("expected second enable to succeed, got %v", err)
	}

	if len(env.state.Projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(env.state.Projects))
	}
}

func TestDaemonServiceEnableReturnsErrorWhenSpawnFails(t *testing.T) {
	projectDir := t.TempDir()
	createIndexFile(t, projectDir)

	env := newDaemonTestEnv(t, nil)
	env.spawner.EXPECT().SpawnWatchProcess(projectDir).Return(0, errors.New("spawn failed")).Times(1)

	err := env.service().Enable(projectDir)
	if err == nil {
		t.Fatal("expected error when spawn fails, got nil")
	}
}

func TestDaemonServiceDisableReturnsErrorWhenStateIsNil(t *testing.T) {
	// newDaemonTestEnv(t, nil) initialises state to nil → ReadState returns nil
	env := newDaemonTestEnv(t, nil)

	err := env.service().Disable(t.TempDir())
	if err == nil {
		t.Fatal("expected error when state is nil, got nil")
	}
}

func TestDaemonServiceDisableReturnsErrorWhenReadStateFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	repo := mocks.NewMockDaemonRepository(ctrl)
	tree := mocks.NewMockProjectTree(ctrl)
	output := mocks.NewMockTextOutput(ctrl)
	initCmd := mocks.NewMockInitCommandInterface(ctrl)
	spawner := mocks.NewMockProcessSpawner(ctrl)

	repo.EXPECT().ReadState().Return(nil, errors.New("state read error")).AnyTimes()

	svc := daemon.NewDaemonService(repo, tree, output, initCmd, spawner)
	err := svc.Disable(t.TempDir())
	if err == nil {
		t.Fatal("expected error when ReadState fails, got nil")
	}
}

func TestDaemonServiceDisableReturnsErrorForUnknownProject(t *testing.T) {
	known := t.TempDir()
	unknown := t.TempDir()
	env := newDaemonTestEnv(t, &domain.DaemonState{
		Projects: []domain.MonitoredProject{monitoredProject(known, 99)},
	})

	err := env.service().Disable(unknown)
	if err == nil {
		t.Fatal("expected error for unmonitored project, got nil")
	}
}

func TestDaemonServiceStatusWithDisabledProject(t *testing.T) {
	projectDir := t.TempDir()
	env := newDaemonTestEnv(t, &domain.DaemonState{
		Projects: []domain.MonitoredProject{
			{Path: projectDir, PID: 0, Enabled: false},
		},
	})

	err := env.service().Status()
	if err != nil {
		t.Fatalf("expected status to succeed for disabled project, got %v", err)
	}
}

func TestDaemonServiceEnableUsesExistingIndexIDXFile(t *testing.T) {
	projectDir := t.TempDir()
	createIndexFile(t, projectDir)

	env := newDaemonTestEnv(t, nil)
	env.expectSpawn(projectDir)

	err := env.service().Enable(projectDir)
	if err != nil {
		t.Fatalf("expected enable to succeed, got %v", err)
	}

	if len(env.lines) > 0 && env.lines[0] == "ℹ️  Index not found. Creating initial index..." {
		t.Fatalf("expected existing .idx/index.idx to skip auto-init, got lines %#v", env.lines)
	}
}
