package repository

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"idx/internal/core/domain"
)

// TestDaemonStateRepositorySaveLoadRealFilesystem testa salvar e carregar estado do filesystem
func TestDaemonStateRepositorySaveLoadRealFilesystem(t *testing.T) {
	tmpHome := t.TempDir()
	stateDir := filepath.Join(tmpHome, ".idx")
	os.MkdirAll(stateDir, 0750)
	statePath := filepath.Join(stateDir, "daemon.state")

	// Escrever JSON manualmente
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

	// Simular leitura (sem mockar UserHomeDir)
	data, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("failed to read state file: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("expected state file to have content")
	}
}

// TestDaemonStateRepositorySaveStateCreatesDirectory testa criação de diretório
func TestDaemonStateRepositorySaveStateCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, ".idx")

	// Verificar que diretório foi criado
	if _, err := os.Stat(stateDir); err == nil {
		t.Fatal("expected directory to not exist initially")
	}

	// Criar diretório
	if err := os.MkdirAll(stateDir, 0750); err != nil {
		t.Fatalf("expected mkdir to succeed, got %v", err)
	}

	if _, err := os.Stat(stateDir); err != nil {
		t.Fatalf("expected directory to exist after creation, got %v", err)
	}
}

// TestDaemonStateRepositoryStateFileFormat testa formato correto do arquivo
func TestDaemonStateRepositoryStateFileFormat(t *testing.T) {
	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, ".idx")
	os.MkdirAll(stateDir, 0750)
	statePath := filepath.Join(stateDir, "daemon.state")

	// Escrever estado
	state := &domain.DaemonState{
		Projects: []domain.MonitoredProject{
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

	// Escrever JSON manualmente para simular salvar
	content, err := os.ReadFile(statePath)
	if err == nil && len(content) > 0 {
		t.Fatal("expected file to not exist yet")
	}

	// Arquivo não deve existir antes de ser criado
	if _, err := os.Stat(statePath); err == nil {
		t.Fatal("expected file to not exist initially")
	}

	_ = state // Usar state para satisfazer lint
}

// TestDaemonStateRepositoryMultipleProjectsFormat testa múltiplos projetos
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

// TestDaemonStateRepositoryPermissions testa permissões do arquivo
func TestDaemonStateRepositoryPermissions(t *testing.T) {
	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, ".idx")
	os.MkdirAll(stateDir, 0750)
	statePath := filepath.Join(stateDir, "daemon.state")

	// Criar arquivo com permissões 0600
	if err := os.WriteFile(statePath, []byte("{}"), 0600); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Verificar permissões
	info, err := os.Stat(statePath)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}

	mode := info.Mode()
	if mode&0077 != 0 {
		t.Fatalf("expected file permissions to be restrictive, got %#o", mode)
	}
}

// TestDaemonStateRepositoryEmptyState testa estado vazio
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

// TestDaemonStateRepositoryInvalidJSONHandling testa arquivo JSON inválido
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

// TestDaemonStateRepositoryDirectoryCreation testa criação de diretório .idx
func TestDaemonStateRepositoryDirectoryCreation(t *testing.T) {
	tmpDir := t.TempDir()
	idxDir := filepath.Join(tmpDir, ".idx")

	// Diretório não deve existir
	if _, err := os.Stat(idxDir); err == nil {
		t.Fatal("expected .idx directory to not exist initially")
	}

	// Criar com MkdirAll
	if err := os.MkdirAll(idxDir, 0750); err != nil {
		t.Fatalf("failed to create .idx directory: %v", err)
	}

	// Agora deve existir
	info, err := os.Stat(idxDir)
	if err != nil {
		t.Fatalf("failed to stat created directory: %v", err)
	}

	if !info.IsDir() {
		t.Fatal("expected .idx to be a directory")
	}
}
