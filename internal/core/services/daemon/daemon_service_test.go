package daemon_test

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"idx/internal/core/domain"
	"idx/internal/core/services/daemon"
)

// Fakes para testes
type fakeDaemonRepository struct {
	state          *domain.DaemonState
	saveError      error
	readError      error
	updatePIDError error
}

func (f *fakeDaemonRepository) ReadState() (*domain.DaemonState, error) {
	if f.readError != nil {
		return nil, f.readError
	}
	if f.state == nil {
		return nil, nil
	}
	// Retorna uma cópia para não alterar estado
	return &domain.DaemonState{
		Projects:  append([]domain.MonitoredProject{}, f.state.Projects...),
		UpdatedAt: f.state.UpdatedAt,
	}, nil
}

func (f *fakeDaemonRepository) SaveState(state *domain.DaemonState) error {
	if f.saveError != nil {
		return f.saveError
	}
	f.state = state
	return nil
}

func (f *fakeDaemonRepository) UpdateProjectPID(projectPath string, pid int) error {
	if f.updatePIDError != nil {
		return f.updatePIDError
	}
	if f.state != nil {
		for i, p := range f.state.Projects {
			if p.Path == projectPath {
				f.state.Projects[i].PID = pid
				break
			}
		}
	}
	return nil
}

type fakeProjectTree struct {
	currentDir string
	gitRoot    string
	existsMap  map[string]bool
}

func (f *fakeProjectTree) CurrentDir() (string, error) {
	return f.currentDir, nil
}

func (f *fakeProjectTree) FindGitRoot(currentPath string) (string, error) {
	return f.gitRoot, nil
}

func (f *fakeProjectTree) Exists(path string) (bool, error) {
	if exists, ok := f.existsMap[path]; ok {
		return exists, nil
	}
	return false, nil
}

func (f *fakeProjectTree) ReadDir(path string) ([]domain.DirectoryEntry, error) {
	return []domain.DirectoryEntry{}, nil
}

func (f *fakeProjectTree) RemoveAll(path string) error {
	return nil
}

func (f *fakeProjectTree) WriteFile(path string, content []byte) error {
	return nil
}

type fakeTextOutput struct {
	lines []string
}

func (f *fakeTextOutput) WriteLine(line string) error {
	f.lines = append(f.lines, line)
	return nil
}

type fakeInitCommand struct {
	callCount   int
	projectPath string
	returnError error
	callPaths   []string
}

func (f *fakeInitCommand) RunFromPath(projectPath string) error {
	f.callCount++
	f.callPaths = append(f.callPaths, projectPath)
	f.projectPath = projectPath
	return f.returnError
}

// Tests
func TestDaemonServiceEnableCreatesNewProject(t *testing.T) {
	projectDir := t.TempDir()

	repo := &fakeDaemonRepository{}
	tree := &fakeProjectTree{
		currentDir: projectDir,
		gitRoot:    projectDir,
		existsMap: map[string]bool{
			projectDir: true,
			filepath.Join(projectDir, ".idx", "index.gob"): true,
		},
	}
	output := &fakeTextOutput{}
	initCmd := &fakeInitCommand{}

	service := daemon.NewDaemonService(repo, tree, output, initCmd)

	err := service.Enable(projectDir)
	if err != nil {
		t.Fatalf("expected enable to succeed, got %v", err)
	}

	state, _ := repo.ReadState()
	if len(state.Projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(state.Projects))
	}
	if state.Projects[0].Path != projectDir {
		t.Fatalf("expected path %s, got %s", projectDir, state.Projects[0].Path)
	}
	if !state.Projects[0].Enabled {
		t.Fatal("expected project to be enabled")
	}
}

func TestDaemonServiceEnableAutoInitsWhenIndexMissing(t *testing.T) {
	projectDir := t.TempDir()

	repo := &fakeDaemonRepository{}
	tree := &fakeProjectTree{
		currentDir: projectDir,
		gitRoot:    projectDir,
		existsMap: map[string]bool{
			projectDir: true,
			filepath.Join(projectDir, ".idx", "index.gob"): false, // Não existe
		},
	}
	output := &fakeTextOutput{}
	initCmd := &fakeInitCommand{}

	service := daemon.NewDaemonService(repo, tree, output, initCmd)

	err := service.Enable(projectDir)
	if err != nil {
		t.Fatalf("expected enable to succeed, got %v", err)
	}

	if initCmd.callCount != 1 {
		t.Fatalf("expected init to be called once, got %d times", initCmd.callCount)
	}
	if initCmd.projectPath != projectDir {
		t.Fatalf("expected init to be called with %s, got %s", projectDir, initCmd.projectPath)
	}
}

func TestDaemonServiceEnableReturnsErrorWhenPathDoesNotExist(t *testing.T) {
	repo := &fakeDaemonRepository{}
	tree := &fakeProjectTree{
		currentDir: "/home/user",
		gitRoot:    "/home/user",
	}
	output := &fakeTextOutput{}
	initCmd := &fakeInitCommand{}

	service := daemon.NewDaemonService(repo, tree, output, initCmd)

	// Usar caminho que não existe no filesystem
	err := service.Enable("/nonexistent/path")
	if err == nil {
		t.Fatal("expected error for nonexistent path, got nil")
	}
}

func TestDaemonServiceEnableReturnsErrorWhenProjectAlreadyMonitored(t *testing.T) {
	existingState := &domain.DaemonState{
		Projects: []domain.MonitoredProject{
			{
				Path:      "/home/user/project",
				PID:       1234,
				Enabled:   true,
				StartedAt: time.Now(),
				LastSync:  time.Now(),
			},
		},
		UpdatedAt: time.Now(),
	}

	repo := &fakeDaemonRepository{state: existingState}
	tree := &fakeProjectTree{
		currentDir: "/home/user",
		gitRoot:    "/home/user/project",
		existsMap: map[string]bool{
			"/home/user/project/.idx/index.gob": true,
		},
	}
	output := &fakeTextOutput{}
	initCmd := &fakeInitCommand{}

	service := daemon.NewDaemonService(repo, tree, output, initCmd)

	err := service.Enable("/home/user/project")
	if err == nil {
		t.Fatal("expected error for already monitored project, got nil")
	}
}

func TestDaemonServiceEnableReturnsErrorWhenInitFails(t *testing.T) {
	repo := &fakeDaemonRepository{}
	tree := &fakeProjectTree{
		currentDir: "/home/user",
		gitRoot:    "/home/user/project",
		existsMap: map[string]bool{
			"/home/user/project/.idx/index.gob": false,
		},
	}
	output := &fakeTextOutput{}
	initCmd := &fakeInitCommand{returnError: fmt.Errorf("init failed")}

	service := daemon.NewDaemonService(repo, tree, output, initCmd)

	err := service.Enable("/home/user/project")
	if err == nil {
		t.Fatal("expected error when init fails, got nil")
	}
}

func TestDaemonServiceDisableRemovesProject(t *testing.T) {
	existingState := &domain.DaemonState{
		Projects: []domain.MonitoredProject{
			{
				Path:      "/home/user/project",
				PID:       9999, // PID que não existe
				Enabled:   true,
				StartedAt: time.Now(),
				LastSync:  time.Now(),
			},
		},
		UpdatedAt: time.Now(),
	}

	repo := &fakeDaemonRepository{state: existingState}
	tree := &fakeProjectTree{}
	output := &fakeTextOutput{}
	initCmd := &fakeInitCommand{}

	service := daemon.NewDaemonService(repo, tree, output, initCmd)

	err := service.Disable("/home/user/project")
	if err != nil {
		t.Fatalf("expected disable to succeed, got %v", err)
	}

	state, _ := repo.ReadState()
	if len(state.Projects) != 0 {
		t.Fatalf("expected 0 projects after disable, got %d", len(state.Projects))
	}
}

func TestDaemonServiceDisableReturnsErrorWhenProjectNotMonitored(t *testing.T) {
	repo := &fakeDaemonRepository{state: &domain.DaemonState{Projects: []domain.MonitoredProject{}}}
	tree := &fakeProjectTree{}
	output := &fakeTextOutput{}
	initCmd := &fakeInitCommand{}

	service := daemon.NewDaemonService(repo, tree, output, initCmd)

	err := service.Disable("/nonexistent/project")
	if err == nil {
		t.Fatal("expected error when project not monitored, got nil")
	}
}

func TestDaemonServiceStatusReturnsEmptyWhenNoProjectsMonitored(t *testing.T) {
	repo := &fakeDaemonRepository{}
	tree := &fakeProjectTree{}
	output := &fakeTextOutput{}
	initCmd := &fakeInitCommand{}

	service := daemon.NewDaemonService(repo, tree, output, initCmd)

	err := service.Status()
	if err != nil {
		t.Fatalf("expected status to succeed, got %v", err)
	}

	if len(output.lines) != 1 {
		t.Fatalf("expected 1 output line, got %d", len(output.lines))
	}
	if output.lines[0] != "❌ No projects being monitored" {
		t.Fatalf("expected 'no projects' message, got %s", output.lines[0])
	}
}

func TestDaemonServiceStatusListsMonitoredProjects(t *testing.T) {
	existingState := &domain.DaemonState{
		Projects: []domain.MonitoredProject{
			{
				Path:      "/home/user/project1",
				PID:       9999,
				Enabled:   true,
				StartedAt: time.Date(2026, 4, 27, 15, 30, 0, 0, time.UTC),
				LastSync:  time.Now(),
			},
			{
				Path:      "/home/user/project2",
				PID:       8888,
				Enabled:   true,
				StartedAt: time.Date(2026, 4, 27, 16, 45, 0, 0, time.UTC),
				LastSync:  time.Now(),
			},
		},
		UpdatedAt: time.Now(),
	}

	repo := &fakeDaemonRepository{state: existingState}
	tree := &fakeProjectTree{}
	output := &fakeTextOutput{}
	initCmd := &fakeInitCommand{}

	service := daemon.NewDaemonService(repo, tree, output, initCmd)

	err := service.Status()
	if err != nil {
		t.Fatalf("expected status to succeed, got %v", err)
	}

	if len(output.lines) < 2 {
		t.Fatalf("expected at least 2 output lines, got %d", len(output.lines))
	}
	if output.lines[0] != "📊 Monitored Projects:" {
		t.Fatalf("expected header, got %s", output.lines[0])
	}
}

func TestDaemonServiceEnableMultipleProjects(t *testing.T) {
	projectDir1 := t.TempDir()
	projectDir2 := t.TempDir()

	repo := &fakeDaemonRepository{}
	tree := &fakeProjectTree{
		currentDir: projectDir1,
		existsMap: map[string]bool{
			projectDir1: true,
			filepath.Join(projectDir1, ".idx", "index.gob"): true,
			projectDir2: true,
			filepath.Join(projectDir2, ".idx", "index.gob"): true,
		},
	}
	output := &fakeTextOutput{}
	initCmd := &fakeInitCommand{}

	service := daemon.NewDaemonService(repo, tree, output, initCmd)

	err := service.Enable(projectDir1)
	if err != nil {
		t.Fatalf("expected first enable to succeed, got %v", err)
	}

	err = service.Enable(projectDir2)
	if err != nil {
		t.Fatalf("expected second enable to succeed, got %v", err)
	}

	state, _ := repo.ReadState()
	if len(state.Projects) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(state.Projects))
	}
}
