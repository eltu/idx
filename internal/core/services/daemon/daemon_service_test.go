package daemon_test

import (
	"errors"
	"path/filepath"
	"testing"

	"idx/internal/core/domain"
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
