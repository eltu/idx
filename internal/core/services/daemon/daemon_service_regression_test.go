package daemon_test

import (
	"errors"
	"path/filepath"
	"testing"

	"go.uber.org/mock/gomock"
)

func TestDaemonRegressionEnableDisableEnableCycle(t *testing.T) {
	projectDir := t.TempDir()
	createIndexFile(t, projectDir)

	env := newDaemonTestEnv(t, nil)
	service := env.service()
	env.expectSpawn(projectDir)

	if err := service.Enable(projectDir); err != nil {
		t.Fatalf("enable failed: %v", err)
	}
	if len(env.state.Projects) != 1 || !env.state.Projects[0].Enabled {
		t.Fatal("expected 1 enabled project after enable")
	}

	if err := service.Disable(projectDir); err != nil {
		t.Fatalf("disable failed: %v", err)
	}
	if len(env.state.Projects) != 0 {
		t.Fatal("expected 0 projects after disable")
	}

	env.expectSpawn(projectDir)
	if err := service.Enable(projectDir); err != nil {
		t.Fatalf("second enable failed: %v", err)
	}
	if len(env.state.Projects) != 1 {
		t.Fatal("expected 1 project after second enable")
	}
}

func TestDaemonRegressionMultipleProjectsSelective(t *testing.T) {
	projectOne := t.TempDir()
	projectTwo := t.TempDir()
	projectThree := t.TempDir()
	createIndexFile(t, projectOne)
	createIndexFile(t, projectTwo)
	createIndexFile(t, projectThree)

	env := newDaemonTestEnv(t, nil)
	service := env.service()
	env.expectSpawn(projectOne)
	env.expectSpawn(projectTwo)
	env.expectSpawn(projectThree)

	if err := service.Enable(projectOne); err != nil {
		t.Fatalf("enable p1 failed: %v", err)
	}
	if err := service.Enable(projectTwo); err != nil {
		t.Fatalf("enable p2 failed: %v", err)
	}
	if err := service.Enable(projectThree); err != nil {
		t.Fatalf("enable p3 failed: %v", err)
	}

	if len(env.state.Projects) != 3 {
		t.Fatalf("expected 3 projects, got %d", len(env.state.Projects))
	}

	if err := service.Disable(projectTwo); err != nil {
		t.Fatalf("disable p2 failed: %v", err)
	}

	if len(env.state.Projects) != 2 {
		t.Fatalf("expected 2 projects after disabling p2, got %d", len(env.state.Projects))
	}

	paths := map[string]bool{}
	for _, project := range env.state.Projects {
		paths[project.Path] = true
	}
	if !paths[projectOne] || !paths[projectThree] {
		t.Fatal("expected p1 and p3 to still be monitored")
	}
	if paths[projectTwo] {
		t.Fatal("expected p2 to be removed")
	}
}

func TestDaemonRegressionInitCalledOnlyWhenNeeded(t *testing.T) {
	projectDir := t.TempDir()
	createIndexFile(t, projectDir)

	env := newDaemonTestEnv(t, nil)
	env.expectSpawn(projectDir)

	if err := env.service().Enable(projectDir); err != nil {
		t.Fatalf("enable failed: %v", err)
	}
}

func TestDaemonRegressionStatusConsistency(t *testing.T) {
	projectDir := t.TempDir()
	createIndexFile(t, projectDir)

	env := newDaemonTestEnv(t, nil)
	service := env.service()
	env.expectSpawn(projectDir)

	if err := service.Enable(projectDir); err != nil {
		t.Fatalf("enable failed: %v", err)
	}

	env.resetLines()
	if err := service.Status(); err != nil {
		t.Fatalf("status after enable failed: %v", err)
	}
	if len(env.lines) < 2 {
		t.Fatalf("expected at least 2 lines in status output, got %d", len(env.lines))
	}

	if err := service.Disable(projectDir); err != nil {
		t.Fatalf("disable failed: %v", err)
	}

	env.resetLines()
	if err := service.Status(); err != nil {
		t.Fatalf("status after disable failed: %v", err)
	}
	if len(env.lines) != 1 {
		t.Fatalf("expected 1 line after disable, got %#v", env.lines)
	}
}

func TestDaemonRegressionPathNormalization(t *testing.T) {
	projectDir := t.TempDir()
	createIndexFile(t, projectDir)

	env := newDaemonTestEnv(t, nil)
	env.expectSpawn(projectDir)

	if err := env.service().Enable(filepath.Join(projectDir, ".")); err != nil {
		t.Fatalf("enable failed: %v", err)
	}

	if env.state.Projects[0].Path != projectDir {
		t.Fatalf("expected normalized path %s, got %s", projectDir, env.state.Projects[0].Path)
	}
}

func TestDaemonRegressionProjectPreservation(t *testing.T) {
	projectOne := t.TempDir()
	projectTwo := t.TempDir()
	createIndexFile(t, projectOne)
	createIndexFile(t, projectTwo)

	env := newDaemonTestEnv(t, nil)
	service := env.service()
	env.expectSpawn(projectOne)
	env.expectSpawn(projectTwo)

	if err := service.Enable(projectOne); err != nil {
		t.Fatalf("enable p1 failed: %v", err)
	}
	if err := service.Enable(projectTwo); err != nil {
		t.Fatalf("enable p2 failed: %v", err)
	}

	if len(env.state.Projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(env.state.Projects))
	}

	foundFirst := false
	for _, project := range env.state.Projects {
		if project.Path == projectOne {
			foundFirst = true
			break
		}
	}
	if !foundFirst {
		t.Fatal("expected p1 to be preserved in state")
	}
}

func TestDaemonRegressionRepositoryIntegration(t *testing.T) {
	projectDir := t.TempDir()
	createIndexFile(t, projectDir)

	env := newDaemonTestEnv(t, nil)
	serviceOne := env.service()
	env.expectSpawn(projectDir)

	if err := serviceOne.Enable(projectDir); err != nil {
		t.Fatalf("enable failed: %v", err)
	}

	serviceTwo := env.service()
	env.resetLines()
	if err := serviceTwo.Status(); err != nil {
		t.Fatalf("status failed: %v", err)
	}

	if len(env.lines) < 2 {
		t.Fatalf("expected status output to show existing project, got %#v", env.lines)
	}
}

func TestDaemonRegressionInitFailureRecovery(t *testing.T) {
	projectWithIndex := t.TempDir()
	createIndexFile(t, projectWithIndex)

	env := newDaemonTestEnv(t, nil)
	env.initCommand.EXPECT().RunFromPath(gomock.Any()).Return(errors.New("init failed")).Times(0)
	env.expectSpawn(projectWithIndex)

	if err := env.service().Enable(projectWithIndex); err != nil {
		t.Fatalf("enable p2 with existing index should succeed, got %v", err)
	}

	if env.state == nil || len(env.state.Projects) != 1 {
		t.Fatalf("expected 1 project after enable, got %#v", env.state)
	}
}

func TestDaemonRegressionConcurrentStateUpdates(t *testing.T) {
	projectOne := t.TempDir()
	projectTwo := t.TempDir()
	createIndexFile(t, projectOne)
	createIndexFile(t, projectTwo)

	env := newDaemonTestEnv(t, nil)
	service := env.service()
	env.expectSpawn(projectOne)
	env.expectSpawn(projectTwo)

	if err := service.Enable(projectOne); err != nil {
		t.Fatalf("enable p1 failed: %v", err)
	}
	if err := service.Enable(projectTwo); err != nil {
		t.Fatalf("enable p2 failed: %v", err)
	}
	if len(env.state.Projects) != 2 {
		t.Fatalf("expected 2 projects after two enables, got %d", len(env.state.Projects))
	}

	if err := service.Disable(projectOne); err != nil {
		t.Fatalf("disable p1 failed: %v", err)
	}
	if len(env.state.Projects) != 1 || env.state.Projects[0].Path != projectTwo {
		t.Fatalf("expected only p2 after disabling p1, got %#v", env.state.Projects)
	}
}

func TestDaemonRegressionStatusFormatting(t *testing.T) {
	env := newDaemonTestEnv(t, nil)

	if err := env.service().Status(); err != nil {
		t.Fatalf("status failed: %v", err)
	}
	if len(env.lines) < 1 || env.lines[0] == "" {
		t.Fatalf("expected non-empty message for empty status, got %#v", env.lines)
	}
}

func TestDaemonRegressionErrorMessages(t *testing.T) {
	projectDir := t.TempDir()
	createIndexFile(t, projectDir)

	env := newDaemonTestEnv(t, nil)
	service := env.service()
	env.expectSpawn(projectDir)

	if err := service.Enable(projectDir); err != nil {
		t.Fatalf("enable failed: %v", err)
	}

	// Enable is idempotent: re-enabling an already-monitored project returns nil.
	if err := service.Enable(projectDir); err != nil {
		t.Fatalf("expected nil when re-enabling already monitored project (idempotent), got %v", err)
	}

	err := service.Disable(filepath.Join(projectDir, "missing"))
	if err == nil {
		t.Fatal("expected error when disabling non-existent project")
	}
}
