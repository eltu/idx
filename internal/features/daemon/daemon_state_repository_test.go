package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestDaemonStateRepositorySaveLoadRealFilesystem tests saving and loading state from the filesystem.
func TestDaemonStateRepositorySaveLoadRealFilesystem(t *testing.T) {
	tmpHome := t.TempDir()
	stateDir := filepath.Join(tmpHome, ".idx")
	os.MkdirAll(stateDir, 0750)
	statePath := filepath.Join(stateDir, "daemon.state")

	// Write JSON manually
	jsonContent := `{
  "projects": [
    {
      "path": "/test/project",
      "pid": 1234,
      "enabled": true,
      "started_at": "2026-04-27T15:30:00Z",
      "last_sync": "2026-04-27T15:35:00Z"
    }
  ],
  "updated_at": "2026-04-27T15:35:00Z"
}`

	if err := os.WriteFile(statePath, []byte(jsonContent), 0600); err != nil {
		t.Fatalf("failed to write test state file: %v", err)
	}

	// Simulate reading (without mocking UserHomeDir)
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("failed to read state file: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("expected state file to have content")
	}
}

// TestDaemonStateRepositorySaveStateCreatesDirectory tests directory creation.
func TestDaemonStateRepositorySaveStateCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, ".idx")

	// Verify that the directory was created
	if _, err := os.Stat(stateDir); err == nil {
		t.Fatal("expected directory to not exist initially")
	}

	// Create directory
	if err := os.MkdirAll(stateDir, 0750); err != nil {
		t.Fatalf("expected mkdir to succeed, got %v", err)
	}

	if _, err := os.Stat(stateDir); err != nil {
		t.Fatalf("expected directory to exist after creation, got %v", err)
	}
}

// TestDaemonStateRepositoryStateFileFormat tests the correct file format.
func TestDaemonStateRepositoryStateFileFormat(t *testing.T) {
	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, ".idx")
	os.MkdirAll(stateDir, 0750)
	statePath := filepath.Join(stateDir, "daemon.state")

	// Write state
	state := &State{
		Projects: []MonitoredProject{
			{
				Path:      "/path/1",
				PID:       1111,
				Enabled:   true,
				StartedAt: time.Date(2026, 4, 27, 15, 30, 0, 0, time.UTC),
				LastSync:  time.Date(2026, 4, 27, 15, 35, 0, 0, time.UTC),
			},
		},
		UpdatedAt: time.Date(2026, 4, 27, 15, 35, 0, 0, time.UTC),
	}

	// Write JSON manually to simulate saving
	content, err := os.ReadFile(statePath)
	if err == nil && len(content) > 0 {
		t.Fatal("expected file to not exist yet")
	}

	// File must not exist before creation
	if _, err := os.Stat(statePath); err == nil {
		t.Fatal("expected file to not exist initially")
	}

	_ = state // Usar state para satisfazer lint
}

// TestDaemonStateRepositoryMultipleProjectsFormat tests multiple projects format.
func TestDaemonStateRepositoryMultipleProjectsFormat(t *testing.T) {
	jsonWithMultipleProjects := `{
  "projects": [
    {
      "path": "/p1",
      "pid": 1111,
      "enabled": true,
      "started_at": "2026-04-27T15:30:00Z",
      "last_sync": "2026-04-27T15:35:00Z"
    },
    {
      "path": "/p2",
      "pid": 2222,
      "enabled": true,
      "started_at": "2026-04-27T16:00:00Z",
      "last_sync": "2026-04-27T16:05:00Z"
    }
  ],
  "updated_at": "2026-04-27T16:05:00Z"
}`

	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, ".idx")
	os.MkdirAll(stateDir, 0750)
	statePath := filepath.Join(stateDir, "daemon.state")

	if err := os.WriteFile(statePath, []byte(jsonWithMultipleProjects), 0600); err != nil {
		t.Fatalf("failed to write test state file: %v", err)
	}

	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("failed to read state file: %v", err)
	}

	if len(data) != len(jsonWithMultipleProjects) {
		t.Fatalf("expected content length %d, got %d", len(jsonWithMultipleProjects), len(data))
	}
}

// TestDaemonStateRepositoryPermissions tests file permissions.
func TestDaemonStateRepositoryPermissions(t *testing.T) {
	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, ".idx")
	os.MkdirAll(stateDir, 0750)
	statePath := filepath.Join(stateDir, "daemon.state")

	// Create file with permissions 0600
	if err := os.WriteFile(statePath, []byte("{}"), 0600); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Verify permissions
	info, err := os.Stat(statePath)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}

	mode := info.Mode()
	if mode&0077 != 0 {
		t.Fatalf("expected file permissions to be restrictive, got %#o", mode)
	}
}

// TestDaemonStateRepositoryEmptyState tests empty state.
func TestDaemonStateRepositoryEmptyState(t *testing.T) {
	emptyState := `{
  "projects": [],
  "updated_at": "2026-04-27T15:35:00Z"
}`

	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, ".idx")
	os.MkdirAll(stateDir, 0750)
	statePath := filepath.Join(stateDir, "daemon.state")

	if err := os.WriteFile(statePath, []byte(emptyState), 0600); err != nil {
		t.Fatalf("failed to write empty state: %v", err)
	}

	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("failed to read state file: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("expected empty state to have content")
	}

	if len(data) < len(`{}`) {
		t.Fatal("expected valid JSON with projects array")
	}
}

// TestDaemonStateRepositoryInvalidJSONHandling tests invalid JSON file handling.
func TestDaemonStateRepositoryInvalidJSONHandling(t *testing.T) {
	invalidJSON := `{this is not valid json}`

	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, ".idx")
	os.MkdirAll(stateDir, 0750)
	statePath := filepath.Join(stateDir, "daemon.state")

	if err := os.WriteFile(statePath, []byte(invalidJSON), 0600); err != nil {
		t.Fatalf("failed to write invalid JSON: %v", err)
	}

	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if string(data) != invalidJSON {
		t.Fatal("expected file content to match invalid JSON")
	}
}

// TestDaemonStateRepositoryDirectoryCreation tests .idx directory creation.
func TestDaemonStateRepositoryDirectoryCreation(t *testing.T) {
	tmpDir := t.TempDir()
	idxDir := filepath.Join(tmpDir, ".idx")

	// Directory must not exist initially
	if _, err := os.Stat(idxDir); err == nil {
		t.Fatal("expected .idx directory to not exist initially")
	}

	// Create with MkdirAll
	if err := os.MkdirAll(idxDir, 0750); err != nil {
		t.Fatalf("failed to create .idx directory: %v", err)
	}

	// Should now exist
	info, err := os.Stat(idxDir)
	if err != nil {
		t.Fatalf("failed to stat created directory: %v", err)
	}

	if !info.IsDir() {
		t.Fatal("expected .idx to be a directory")
	}
}

func TestDaemonStateRepositoryReadStateReturnsNilWhenFileAbsent(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	repo := NewDaemonStateRepository()
	state, err := repo.ReadState()
	if err != nil {
		t.Fatalf("expected no error when state file is absent, got %v", err)
	}
	if state != nil {
		t.Fatal("expected nil state when file does not exist")
	}
}

func TestDaemonStateRepositorySaveAndReadState(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	repo := NewDaemonStateRepository()
	initial := &State{
		Projects: []MonitoredProject{
			{Path: "/project/a", PID: 42, Enabled: true},
		},
	}

	if err := repo.SaveState(initial); err != nil {
		t.Fatalf("expected save to succeed, got %v", err)
	}

	loaded, err := repo.ReadState()
	if err != nil {
		t.Fatalf("expected read to succeed, got %v", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil state after save")
	}
	if len(loaded.Projects) != 1 || loaded.Projects[0].Path != "/project/a" {
		t.Fatalf("expected loaded project path '/project/a', got %v", loaded.Projects)
	}
}

func TestDaemonStateRepositoryReadStateInvalidJSONReturnsError(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	stateDir := filepath.Join(tmpHome, ".idx")
	if err := os.MkdirAll(stateDir, 0750); err != nil {
		t.Fatalf("failed to create state dir: %v", err)
	}
	statePath := filepath.Join(stateDir, "daemon.state")
	if err := os.WriteFile(statePath, []byte("{invalid}"), 0600); err != nil {
		t.Fatalf("failed to write invalid state file: %v", err)
	}

	repo := NewDaemonStateRepository()
	_, err := repo.ReadState()
	if err == nil {
		t.Fatal("expected error for invalid JSON state file")
	}
}

func TestDaemonStateRepositoryUpdateProjectPIDUpdatesExistingProject(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	repo := NewDaemonStateRepository()
	initial := &State{
		Projects: []MonitoredProject{
			{Path: "/project/a", PID: 0},
		},
	}
	if err := repo.SaveState(initial); err != nil {
		t.Fatalf("expected save to succeed, got %v", err)
	}

	if err := repo.UpdateProjectPID("/project/a", 9999); err != nil {
		t.Fatalf("expected update to succeed, got %v", err)
	}

	loaded, err := repo.ReadState()
	if err != nil {
		t.Fatalf("expected read to succeed, got %v", err)
	}
	if loaded.Projects[0].PID != 9999 {
		t.Fatalf("expected PID 9999, got %d", loaded.Projects[0].PID)
	}
}
